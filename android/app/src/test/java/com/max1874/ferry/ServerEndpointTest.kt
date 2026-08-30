package com.max1874.ferry

import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class ServerEndpointTest {
    @Test fun normalizesOriginAndDefaultPorts() {
        assertEquals("http://example.com", ServerEndpoint.parse(" HTTP://Example.COM:80/ ").origin)
        assertEquals("https://example.com", ServerEndpoint.parse("https://Example.COM:443").origin)
        assertEquals("http://10.0.0.2:42817", ServerEndpoint.parse("http://10.0.0.2:42817").origin)
    }

    @Test fun rejectsAnythingBeyondAnOrigin() {
        listOf(
            "ftp://example.com", "http://user@example.com", "http://example.com/path",
            "http://example.com?x=1", "http://example.com#fragment", "http://example.com:0",
        ).forEach { value ->
            assertThrows(value, IllegalArgumentException::class.java) { ServerEndpoint.parse(value) }
        }
    }

    @Test fun buildsOnlyAbsoluteApiPaths() {
        val endpoint = ServerEndpoint.parse("http://10.0.0.2:42817")
        assertEquals("http://10.0.0.2:42817/api/v1/messages?after=2", endpoint.url("/api/v1/messages", "after=2"))
        assertThrows(IllegalArgumentException::class.java) { endpoint.url("//attacker.test") }
    }
}
