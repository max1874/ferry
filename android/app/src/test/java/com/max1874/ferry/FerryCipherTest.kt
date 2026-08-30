package com.max1874.ferry

import javax.crypto.AEADBadTagException
import javax.crypto.KeyGenerator
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class FerryCipherTest {
    @Test fun roundTripsWithoutStoringPlaintext() {
        val key = KeyGenerator.getInstance("AES").apply { init(256) }.generateKey()
        val encrypted = FerryCipher.encrypt(key, "device-token")
        assertNotEquals("device-token", encrypted)
        assertEquals("device-token", FerryCipher.decrypt(key, encrypted))
    }

    @Test fun rejectsTamperingAndTheWrongKey() {
        val generator = KeyGenerator.getInstance("AES").apply { init(256) }
        val key = generator.generateKey()
        val encrypted = FerryCipher.encrypt(key, "device-token")
        assertThrows(AEADBadTagException::class.java) { FerryCipher.decrypt(generator.generateKey(), encrypted) }

        val bytes = java.util.Base64.getDecoder().decode(encrypted)
        bytes[bytes.lastIndex] = (bytes.last().toInt() xor 1).toByte()
        val tampered = java.util.Base64.getEncoder().encodeToString(bytes)
        assertThrows(AEADBadTagException::class.java) { FerryCipher.decrypt(key, tampered) }
    }
}
