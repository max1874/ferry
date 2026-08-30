package com.max1874.ferry

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.DisposableHandle
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.InternalCoroutinesApi
import kotlinx.coroutines.Job
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URL
import java.util.UUID

interface FerryService {
    suspend fun join(endpoint: ServerEndpoint, deviceName: String, password: String): AccessClaim
    suspend fun session(endpoint: ServerEndpoint, token: String): FerryDevice
    suspend fun messages(endpoint: ServerEndpoint, token: String, after: Long): MessagesPage
    suspend fun sendText(endpoint: ServerEndpoint, token: String, text: String): FerryMessage
    suspend fun sendFile(endpoint: ServerEndpoint, token: String, file: SelectedContent): FerryMessage
    suspend fun download(endpoint: ServerEndpoint, token: String, file: FerryFile, destinationUri: String)
}

class FerryClient(
    private val contentStore: ContentStore,
    private val dispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val openConnection: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) : FerryService {
    override suspend fun join(endpoint: ServerEndpoint, deviceName: String, password: String): AccessClaim {
        val response = jsonRequest(
            endpoint.url("/api/v1/access/join"), "POST", null,
            FerryJson.joinBody(deviceName, password), mapOf("X-Ferry-Device-Kind" to "android"),
        )
        return FerryJson.accessClaim(response)
    }

    override suspend fun session(endpoint: ServerEndpoint, token: String): FerryDevice =
        FerryJson.session(jsonRequest(endpoint.url("/api/v1/session"), "GET", token))

    override suspend fun messages(endpoint: ServerEndpoint, token: String, after: Long): MessagesPage {
        require(after >= 0)
        return FerryJson.messages(
            jsonRequest(
                endpoint.url("/api/v1/messages", "after=$after&limit=${FerryJson.MESSAGE_PAGE_LIMIT}"),
                "GET",
                token,
            ),
        )
    }

    override suspend fun sendText(endpoint: ServerEndpoint, token: String, text: String): FerryMessage =
        FerryJson.message(
            jsonRequest(endpoint.url("/api/v1/messages/text"), "POST", token, FerryJson.textBody(text)),
        )

    override suspend fun sendFile(
        endpoint: ServerEndpoint,
        token: String,
        file: SelectedContent,
    ): FerryMessage = withContext(dispatcher) {
        if (file.size != null && (file.size < 0 || file.size > FerryJson.MAX_FILE_BYTES)) {
            throw FerryApiException(413, "payload_too_large", "Files must be 64 MB or smaller.")
        }
        val boundary = "Ferry-${UUID.randomUUID()}"
        val connection = configured(endpoint.url("/api/v1/messages/file"), "POST", token)
        val cancellation = disconnectOnCancellation(connection)
        connection.setRequestProperty("Content-Type", "multipart/form-data; boundary=$boundary")
        connection.doOutput = true
        connection.setChunkedStreamingMode(BUFFER_BYTES)
        try {
            connection.outputStream.use { output ->
                val safeName = safeFilename(file.name)
                output.write("--$boundary\r\n".toByteArray())
                output.write("Content-Disposition: form-data; name=\"file\"; filename=\"$safeName\"\r\n".toByteArray())
                output.write("Content-Type: ${safeMediaType(file.mediaType)}\r\n\r\n".toByteArray())
                contentStore.open(file.uri).use { input -> copyUpload(input, output) }
                output.write("\r\n--$boundary--\r\n".toByteArray())
            }
            FerryJson.message(readJsonResponse(connection))
        } finally {
            cancellation?.dispose()
            connection.disconnect()
        }
    }

    override suspend fun download(
        endpoint: ServerEndpoint,
        token: String,
        file: FerryFile,
        destinationUri: String,
    ) = withContext(dispatcher) {
        val connection = configured(endpoint.url(file.downloadUrl), "GET", token)
        val cancellation = disconnectOnCancellation(connection)
        try {
            val status = connection.responseCode
            if (status !in 200..299) {
                throw FerryJson.error(readBounded(connection.errorStream, FerryJson.MAX_JSON_BYTES), status)
            }
            var written = 0L
            contentStore.create(destinationUri).use { output ->
                connection.inputStream.use { input ->
                    val buffer = ByteArray(BUFFER_BYTES)
                    while (true) {
                        currentCoroutineContext().ensureActive()
                        val count = input.read(buffer)
                        if (count < 0) break
                        written += count
                        if (written > FerryJson.MAX_FILE_BYTES || written > file.size) {
                            throw FerryProtocolException("Downloaded file is larger than declared.")
                        }
                        output.write(buffer, 0, count)
                    }
                }
            }
            if (written != file.size) throw FerryProtocolException("Downloaded file size does not match the message.")
        } catch (error: Exception) {
            if (contentStore.delete(destinationUri)) throw error
            val message = (error.message ?: "Download failed") +
                " The incomplete destination file could not be removed."
            throw if (error is FerryApiException) FerryApiException(error.status, error.code, message, cleanupFailed = true)
            else FerryProtocolException(message, error)
        } finally {
            cancellation?.dispose()
            connection.disconnect()
        }
    }

    private suspend fun jsonRequest(
        url: String,
        method: String,
        token: String?,
        body: ByteArray? = null,
        headers: Map<String, String> = emptyMap(),
    ): ByteArray = withContext(dispatcher) {
        val connection = configured(url, method, token)
        val cancellation = disconnectOnCancellation(connection)
        headers.forEach(connection::setRequestProperty)
        try {
            if (body != null) {
                connection.doOutput = true
                connection.setRequestProperty("Content-Type", "application/json")
                connection.setFixedLengthStreamingMode(body.size)
                connection.outputStream.use { it.write(body) }
            }
            readJsonResponse(connection)
        } finally {
            cancellation?.dispose()
            connection.disconnect()
        }
    }

    @OptIn(InternalCoroutinesApi::class)
    private suspend fun disconnectOnCancellation(connection: HttpURLConnection): DisposableHandle? {
        val job = currentCoroutineContext()[Job] ?: return null
        return job.invokeOnCompletion(onCancelling = true, invokeImmediately = true) { cause ->
            if (cause is kotlinx.coroutines.CancellationException) connection.disconnect()
        }
    }

    private fun configured(url: String, method: String, token: String?): HttpURLConnection {
        return openConnection(URL(url)).apply {
            requestMethod = method
            connectTimeout = TIMEOUT_MS
            readTimeout = TIMEOUT_MS
            useCaches = false
            instanceFollowRedirects = false
            setRequestProperty("Accept", "application/json")
            if (token != null) setRequestProperty("Authorization", "Bearer $token")
        }
    }

    private fun readJsonResponse(connection: HttpURLConnection): ByteArray {
        val status = connection.responseCode
        val bytes = readBounded(
            if (status in 200..299) connection.inputStream else connection.errorStream,
            FerryJson.MAX_JSON_BYTES,
        )
        if (status !in 200..299) throw FerryJson.error(bytes, status)
        val mediaType = connection.contentType?.substringBefore(';')?.trim()?.lowercase()
        if (mediaType != "application/json") throw FerryProtocolException("Ferry Server returned non-JSON data.")
        return bytes
    }

    private suspend fun copyUpload(input: InputStream, output: java.io.OutputStream) {
        var written = 0L
        val buffer = ByteArray(BUFFER_BYTES)
        while (true) {
            currentCoroutineContext().ensureActive()
            val count = input.read(buffer)
            if (count < 0) return
            written += count
            if (written > FerryJson.MAX_FILE_BYTES) {
                throw FerryApiException(413, "payload_too_large", "Files must be 64 MB or smaller.")
            }
            output.write(buffer, 0, count)
        }
    }

    private fun readBounded(input: InputStream?, limit: Int): ByteArray {
        if (input == null) return ByteArray(0)
        input.use {
            val output = ByteArrayOutputStream()
            val buffer = ByteArray(8192)
            var total = 0
            while (true) {
                val count = it.read(buffer)
                if (count < 0) return output.toByteArray()
                total += count
                if (total > limit) throw FerryProtocolException("Ferry response is too large.")
                output.write(buffer, 0, count)
            }
        }
    }

    private fun safeFilename(value: String): String {
        val leaf = value.substringAfterLast('/').substringAfterLast('\\')
        return leaf.replace("\\", "_").replace("\"", "_").replace("\r", "_").replace("\n", "_")
            .takeIf { it.isNotBlank() } ?: "file"
    }

    private fun safeMediaType(value: String): String = value.takeIf {
        it.length <= 127 && it.count { character -> character == '/' } == 1 &&
            it.none { character -> character <= ' ' || character == '\u007f' }
    } ?: "application/octet-stream"

    companion object {
        private const val BUFFER_BYTES = 256 shl 10
        private const val TIMEOUT_MS = 15_000
    }
}
