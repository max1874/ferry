package com.max1874.ferry

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test

class FerryJsonTest {
    private val id1 = "0123456789abcdef0123456789abcdef"
    private val id2 = "fedcba9876543210fedcba9876543210"

    @Test fun encodesJoinWithoutChangingUserInput() {
        val body = JSONObject(FerryJson.joinBody("Pixel 9", "secret").toString(Charsets.UTF_8))
        assertEquals(setOf("device_name", "password"), body.keySet())
        assertEquals("Pixel 9", body.getString("device_name"))
        assertEquals("secret", body.getString("password"))
    }

    @Test fun decodesOrderedTextAndFileMessages() {
        val page = FerryJson.messages(
            """{"messages":[
              {"id":"$id1","sequence":1,"sender_name":"iPhone","sender_kind":"iphone","created_at":"2026-08-30T10:00:00Z","kind":"text","text":"hello"},
              {"id":"$id2","sequence":2,"sender_name":"Mac","sender_kind":"mac","created_at":"2026-08-30T10:00:01Z","kind":"file","file":{"name":"a.png","media_type":"image/png","size":12,"download_url":"/api/v1/files/$id2"}}
            ],"next_cursor":2}""".toByteArray(),
        )
        assertEquals(2, page.messages.size)
        assertEquals("hello", (page.messages[0] as FerryMessage.Text).text)
        assertEquals("a.png", (page.messages[1] as FerryMessage.File).file.name)
        assertEquals(2, page.nextCursor)
    }

    @Test fun rejectsPayloadConfusionAndUntrustedDownloadPaths() {
        val confused = """{"id":"$id1","sequence":1,"sender_name":"x","sender_kind":"android","created_at":"2026-08-30T10:00:00Z","kind":"text","text":"x","file":{}}"""
        assertThrows(FerryProtocolException::class.java) { FerryJson.message(confused.toByteArray()) }

        val redirected = """{"id":"$id1","sequence":1,"sender_name":"x","sender_kind":"browser","created_at":"2026-08-30T10:00:00Z","kind":"file","file":{"name":"x","media_type":"text/plain","size":1,"download_url":"//attacker.test/x"}}"""
        assertThrows(FerryProtocolException::class.java) { FerryJson.message(redirected.toByteArray()) }
    }

    @Test fun convertsMalformedErrorBodiesToBoundedGenericError() {
        val error = FerryJson.error("not-json".toByteArray(), 502)
        assertEquals(502, error.status)
        assertEquals("http_error", error.code)
        assertTrue(error.message!!.contains("502"))
    }

    @Test fun rejectsCredentialsThatDoNotMatchTheServerContract() {
        val device = """{"id":"$id1","name":"Pixel","kind":"android","created_at":"2026-08-30T10:00:00Z"}"""
        listOf("short", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPB").forEach { token ->
            assertThrows(FerryProtocolException::class.java) {
                FerryJson.accessClaim("""{"device":$device,"token":"$token"}""".toByteArray())
            }
        }
    }
}
