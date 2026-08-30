package com.max1874.ferry
import java.net.URI
import java.util.Locale
class ServerEndpoint private constructor(val origin: String) {
    fun url(path: String, query: String = ""): String {
        require(path.startsWith("/") && !path.startsWith("//"))
        return origin + path + if (query.isEmpty()) "" else "?$query"
    }
    companion object {
        fun parse(input: String): ServerEndpoint {
            val trimmed = input.trim()
            val uri = try { URI(trimmed) } catch (_: Exception) { throw invalidEndpoint() }
            val scheme = uri.scheme?.lowercase(Locale.ROOT)
            val host = uri.host?.lowercase(Locale.ROOT)
            if (scheme !in setOf("http", "https") || host.isNullOrEmpty() || uri.rawUserInfo != null ||
                uri.rawQuery != null || uri.rawFragment != null || (uri.rawPath != "" && uri.rawPath != "/") ||
                uri.port !in -1..65535 || uri.port == 0
            ) throw invalidEndpoint()
            val port = when {
                scheme == "http" && uri.port == 80 -> -1
                scheme == "https" && uri.port == 443 -> -1
                else -> uri.port
            }
            val normalized = URI(scheme, null, host, port, null, null, null).toASCIIString()
            return ServerEndpoint(normalized)
        }
        private fun invalidEndpoint() = IllegalArgumentException(
            "Enter an HTTP or HTTPS Server origin, for example http://10.0.0.2:42817.",
        )
    }
}
