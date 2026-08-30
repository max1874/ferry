package com.max1874.ferry

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.cancelAndJoin
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class FerryClientTest {
    private val id = "0123456789abcdef0123456789abcdef"

    @Test fun joinUsesAndroidIdentityAndExactJsonContract() {
        val token = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQ"
        val response = """{"device":{"id":"$id","name":"Pixel","kind":"android","created_at":"2026-08-30T10:00:00Z"},"token":"$token"}"""
        val connection = FakeConnection(200, response.toByteArray(), "application/json")
        val client = FerryClient(MemoryContentStore(), openConnection = { connection })

        val claim = runBlocking {
            client.join(ServerEndpoint.parse("http://10.0.0.2:42817"), "Pixel", "secret")
        }

        assertEquals("POST", connection.requestMethod)
        assertEquals(false, connection.instanceFollowRedirects)
        assertEquals("android", connection.getRequestProperty("X-Ferry-Device-Kind"))
        val body = JSONObject(connection.sent.toString(Charsets.UTF_8))
        assertEquals(setOf("device_name", "password"), body.keySet())
        assertEquals("Pixel", body.getString("device_name"))
        assertEquals("secret", body.getString("password"))
        assertEquals(DeviceKind.ANDROID, claim.device.kind)
        assertEquals(token, claim.token)
    }

    @Test fun authenticatedDownloadWritesOnlyTheDeclaredPayload() {
        val connection = FakeConnection(200, "hello".toByteArray(), "application/octet-stream")
        val store = MemoryContentStore()
        val client = FerryClient(store, openConnection = { connection })
        val file = FerryFile("hello.txt", "text/plain", 5, "/api/v1/files/$id")

        runBlocking {
            client.download(ServerEndpoint.parse("http://10.0.0.2:42817"), "token", file, "destination")
        }

        assertEquals("Bearer token", connection.getRequestProperty("Authorization"))
        assertEquals("hello", store.saved.toString(Charsets.UTF_8))
        assertTrue(store.deleted.isEmpty())
    }

    @Test fun sizeMismatchDeletesThePartialDestination() {
        val connection = FakeConnection(200, "short".toByteArray(), "application/octet-stream")
        val store = MemoryContentStore()
        val client = FerryClient(store, openConnection = { connection })
        val file = FerryFile("file.bin", "application/octet-stream", 9, "/api/v1/files/$id")

        runCatching {
            runBlocking {
                client.download(ServerEndpoint.parse("http://10.0.0.2:42817"), "token", file, "destination")
            }
        }.onSuccess { error("Expected a protocol failure") }

        assertEquals(listOf("destination"), store.deleted)
    }

    @Test fun failedCleanupIsPartOfTheVisibleDownloadError() {
        val connection = FakeConnection(200, "short".toByteArray(), "application/octet-stream")
        val client = FerryClient(MemoryContentStore(deleteSucceeds = false), openConnection = { connection })
        val file = FerryFile("file.bin", "application/octet-stream", 9, "/api/v1/files/$id")

        val error = runCatching {
            runBlocking {
                client.download(ServerEndpoint.parse("http://10.0.0.2:42817"), "token", file, "destination")
            }
        }.exceptionOrNull()

        assertTrue(error is FerryProtocolException)
        assertTrue(error!!.message!!.contains("incomplete destination file could not be removed"))
    }

    @Test fun unauthorizedDownloadPreservesTheCleanupFailureFlag() {
        val body = """{"error":{"code":"invalid_token","message":"expired"}}""".toByteArray()
        val connection = FakeConnection(401, body, "application/json")
        val client = FerryClient(MemoryContentStore(deleteSucceeds = false), openConnection = { connection })
        val file = FerryFile("file.bin", "application/octet-stream", 9, "/api/v1/files/$id")

        val error = runCatching {
            runBlocking {
                client.download(ServerEndpoint.parse("http://10.0.0.2:42817"), "token", file, "destination")
            }
        }.exceptionOrNull()

        assertTrue(error is FerryApiException)
        assertEquals(true, (error as FerryApiException).cleanupFailed)
    }

    @Test fun unknownMetadataCannotBypassTheStreamingUploadLimit() {
        val connection = DiscardConnection()
        val client = FerryClient(OversizedContentStore(), openConnection = { connection })

        val error = runCatching {
            runBlocking {
                client.sendFile(
                    ServerEndpoint.parse("http://10.0.0.2:42817"),
                    "token",
                    SelectedContent("content", "large.bin", "application/octet-stream", null),
                )
            }
        }.exceptionOrNull()

        assertTrue(error is FerryApiException)
        assertEquals(413, (error as FerryApiException).status)
    }

    @Test fun boundsAFullPageOfMaximallyEscapedText() {
        val connection = FakeConnection(
            200,
            """{"messages":[],"next_cursor":0}""".toByteArray(),
            "application/json",
        )
        var requestedUrl = ""
        val client = FerryClient(MemoryContentStore(), openConnection = {
            requestedUrl = it.toString()
            connection
        })

        val page = runBlocking {
            client.messages(ServerEndpoint.parse("http://10.0.0.2:42817"), "token", 0)
        }

        assertTrue(requestedUrl.endsWith("after=0&limit=2"))
        assertTrue(FerryJson.MESSAGE_PAGE_LIMIT * (64 shl 10) * 6 < FerryJson.MAX_JSON_BYTES)
        assertTrue(page.messages.isEmpty())
    }

    @Test fun cancellationDisconnectsBlockingNetworkIo() = runBlocking {
        val connection = BlockingConnection()
        val client = FerryClient(MemoryContentStore(), openConnection = { connection })
        val request = launch(Dispatchers.IO) {
            client.messages(ServerEndpoint.parse("http://10.0.0.2:42817"), "token", 0)
        }
        assertTrue(connection.started.await(2, TimeUnit.SECONDS))

        request.cancelAndJoin()

        assertTrue(connection.disconnected.await(2, TimeUnit.SECONDS))
    }

    private open class FakeConnection(
        private val status: Int,
        val response: ByteArray,
        private val responseType: String,
    ) : HttpURLConnection(URL("http://127.0.0.1")) {
        val sent = ByteArrayOutputStream()
        val disconnected = CountDownLatch(1)
        override fun connect() = Unit
        override fun disconnect() { disconnected.countDown() }
        override fun usingProxy() = false
        override fun getResponseCode() = status
        override fun getContentType() = responseType
        override fun getInputStream(): java.io.InputStream = ByteArrayInputStream(response)
        override fun getErrorStream() = if (status in 200..299) null else ByteArrayInputStream(response)
        override fun getOutputStream(): java.io.OutputStream = sent
    }

    private class BlockingConnection : FakeConnection(200, ByteArray(0), "application/json") {
        val started = CountDownLatch(1)
        override fun getInputStream() = object : java.io.InputStream() {
            override fun read(): Int {
                started.countDown()
                disconnected.await(2, TimeUnit.SECONDS)
                return -1
            }
        }
    }

    private class DiscardConnection : FakeConnection(200, ByteArray(0), "application/json") {
        override fun getOutputStream() = object : java.io.OutputStream() {
            override fun write(value: Int) = Unit
            override fun write(value: ByteArray, offset: Int, length: Int) = Unit
        }
    }

    private class MemoryContentStore(private val deleteSucceeds: Boolean = true) : ContentStore {
        var saved = ByteArray(0)
        val deleted = mutableListOf<String>()
        override suspend fun describe(uri: String) = SelectedContent(uri, "file", "application/octet-stream", null)
        override fun open(uri: String) = ByteArrayInputStream(ByteArray(0))
        override fun create(uri: String) = object : ByteArrayOutputStream() {
            override fun close() {
                saved = toByteArray()
                super.close()
            }
        }
        override fun delete(uri: String): Boolean { deleted += uri; return deleteSucceeds }
    }

    private class OversizedContentStore : ContentStore {
        override suspend fun describe(uri: String) = error("unexpected")
        override fun open(uri: String) = object : java.io.InputStream() {
            var remaining = FerryJson.MAX_FILE_BYTES + 1
            override fun read(): Int = if (remaining-- > 0) 0 else -1
            override fun read(buffer: ByteArray, offset: Int, length: Int): Int {
                if (remaining == 0L) return -1
                val count = minOf(remaining, length.toLong()).toInt()
                java.util.Arrays.fill(buffer, offset, offset + count, 0)
                remaining -= count
                return count
            }
        }
        override fun create(uri: String) = error("unexpected")
        override fun delete(uri: String) = true
    }
}
