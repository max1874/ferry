package com.max1874.ferry

import org.json.JSONArray
import org.json.JSONObject
import java.time.Instant
import java.util.Base64

enum class DeviceKind(val wire: String) {
    IPHONE("iphone"), IPAD("ipad"), MAC("mac"), ANDROID("android"), WINDOWS("windows"), BROWSER("browser");

    companion object {
        fun decode(value: String): DeviceKind = entries.firstOrNull { it.wire == value }
            ?: throw FerryProtocolException("Unknown device kind: $value")
    }
}

data class FerryDevice(val id: String, val name: String, val kind: DeviceKind, val createdAt: String)

data class FerryFile(
    val name: String,
    val mediaType: String,
    val size: Long,
    val downloadUrl: String,
)

sealed interface FerryMessage {
    val id: String
    val sequence: Long
    val senderName: String
    val senderKind: DeviceKind
    val createdAt: String

    /**
     * Whether this device sent the message. Clipboard sync uses it to leave its
     * own sends alone; comparing sender names would confuse two devices that
     * happen to share one.
     */
    val isCurrentDevice: Boolean

    data class Text(
        override val id: String,
        override val sequence: Long,
        override val senderName: String,
        override val senderKind: DeviceKind,
        override val createdAt: String,
        val text: String,
        override val isCurrentDevice: Boolean = false,
    ) : FerryMessage

    data class File(
        override val id: String,
        override val sequence: Long,
        override val senderName: String,
        override val senderKind: DeviceKind,
        override val createdAt: String,
        val file: FerryFile,
        override val isCurrentDevice: Boolean = false,
    ) : FerryMessage
}

data class AccessClaim(val device: FerryDevice, val token: String)
data class MessagesPage(val messages: List<FerryMessage>, val nextCursor: Long)

class FerryProtocolException(message: String, cause: Throwable? = null) : Exception(message, cause)
class FerryApiException(val status: Int, val code: String, message: String, val cleanupFailed: Boolean = false) : Exception(message)

object FerryJson {
    private val idPattern = Regex("^[0-9a-f]{32}$")
    const val MAX_FILE_BYTES = 64L shl 20
    const val MESSAGE_PAGE_LIMIT = 2
    const val MAX_JSON_BYTES = 2 shl 20

    fun accessClaim(bytes: ByteArray): AccessClaim = decode(bytes) { root ->
        val token = requiredString(root, "token")
        requireToken(token)
        AccessClaim(device(root.getJSONObject("device")), token)
    }

    fun session(bytes: ByteArray): FerryDevice = decode(bytes) { device(it.getJSONObject("device")) }

    fun messages(bytes: ByteArray): MessagesPage = decode(bytes) { root ->
        val values = root.getJSONArray("messages")
        val messages = buildList(values.length()) {
            for (index in 0 until values.length()) add(message(values.getJSONObject(index)))
        }
        val cursor = root.getLong("next_cursor")
        if (cursor < 0 || messages.zipWithNext().any { (a, b) -> a.sequence >= b.sequence }) {
            throw FerryProtocolException("Invalid message cursor or ordering")
        }
        MessagesPage(messages, cursor)
    }

    fun message(bytes: ByteArray): FerryMessage = decode(bytes, ::message)

    fun error(bytes: ByteArray, status: Int): FerryApiException {
        return try {
            val detail = JSONObject(bytes.toString(Charsets.UTF_8)).getJSONObject("error")
            FerryApiException(status, requiredString(detail, "code"), requiredString(detail, "message"))
        } catch (_: Exception) {
            FerryApiException(status, "http_error", "Ferry Server returned HTTP $status")
        }
    }

    fun joinBody(deviceName: String, password: String): ByteArray = JSONObject()
        .put("device_name", deviceName)
        .put("password", password)
        .toString().toByteArray(Charsets.UTF_8)

    fun textBody(text: String): ByteArray = JSONObject().put("text", text).toString().toByteArray(Charsets.UTF_8)

    private fun device(value: JSONObject): FerryDevice {
        val id = requiredString(value, "id")
        requireId(id)
        val name = requiredString(value, "name")
        val createdAt = requiredString(value, "created_at")
        requireTimestamp(createdAt)
        return FerryDevice(id, name, DeviceKind.decode(requiredString(value, "kind")), createdAt)
    }

    private fun message(value: JSONObject): FerryMessage {
        val id = requiredString(value, "id")
        requireId(id)
        val sequence = value.getLong("sequence")
        if (sequence < 1) throw FerryProtocolException("Invalid message sequence")
        val sender = requiredString(value, "sender_name")
        val senderKind = DeviceKind.decode(requiredString(value, "sender_kind"))
        val createdAt = requiredString(value, "created_at")
        requireTimestamp(createdAt)
        val hasText = value.has("text") && !value.isNull("text")
        val hasFile = value.has("file") && !value.isNull("file")
        val isCurrentDevice = value.optBoolean("is_current_device", false)
        return when (requiredString(value, "kind")) {
            "text" -> {
                if (!hasText || hasFile) throw FerryProtocolException("Text message payload does not match kind")
                FerryMessage.Text(id, sequence, sender, senderKind, createdAt, requiredString(value, "text"), isCurrentDevice)
            }
            "file" -> {
                if (hasText || !hasFile) throw FerryProtocolException("File message payload does not match kind")
                val file = value.getJSONObject("file")
                val size = file.getLong("size")
                if (size < 0 || size > MAX_FILE_BYTES) throw FerryProtocolException("Invalid file size")
                val downloadUrl = requiredString(file, "download_url")
                if (downloadUrl != "/api/v1/files/$id") throw FerryProtocolException("Invalid file download URL")
                FerryMessage.File(
                    id, sequence, sender, senderKind, createdAt,
                    FerryFile(requiredString(file, "name"), requiredString(file, "media_type"), size, downloadUrl),
                    isCurrentDevice,
                )
            }
            else -> throw FerryProtocolException("Unknown message kind")
        }
    }

    private fun requiredString(value: JSONObject, key: String): String {
        val result = value.getString(key)
        if (result.isEmpty()) throw FerryProtocolException("Empty $key")
        return result
    }

    private fun requireId(value: String) {
        if (!idPattern.matches(value)) throw FerryProtocolException("Invalid Ferry ID")
    }

    private fun requireTimestamp(value: String) {
        try {
            Instant.parse(value)
        } catch (error: Exception) {
            throw FerryProtocolException("Invalid timestamp", error)
        }
    }

    private fun requireToken(value: String) {
        val decoded = try { Base64.getUrlDecoder().decode(value) } catch (_: IllegalArgumentException) { null }
        if (decoded?.size != 32 || Base64.getUrlEncoder().withoutPadding().encodeToString(decoded) != value) {
            throw FerryProtocolException("Invalid device credential")
        }
    }

    private fun <T> decode(bytes: ByteArray, block: (JSONObject) -> T): T {
        return try {
            block(JSONObject(bytes.toString(Charsets.UTF_8)))
        } catch (error: FerryProtocolException) {
            throw error
        } catch (error: Exception) {
            throw FerryProtocolException("Invalid Ferry response", error)
        }
    }
}
