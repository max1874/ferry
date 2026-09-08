package com.max1874.ferry

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.security.KeyStore
import java.security.MessageDigest
import java.util.Base64
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

interface CredentialStore {
    fun token(origin: String): String?
    fun save(origin: String, token: String)
    fun remove(origin: String)
}

class AndroidCredentialStore(context: Context) : CredentialStore {
    private val preferences = context.getSharedPreferences("ferry.credentials", Context.MODE_PRIVATE)

    override fun token(origin: String): String? {
        val encoded = preferences.getString(preferenceKey(origin), null) ?: return null
        return FerryCipher.decrypt(key(), encoded)
    }

    override fun save(origin: String, token: String) {
        check(preferences.edit().putString(preferenceKey(origin), FerryCipher.encrypt(key(), token)).commit()) {
            "Could not save this device credential."
        }
    }

    override fun remove(origin: String) {
        check(preferences.edit().remove(preferenceKey(origin)).commit()) { "Could not remove this device credential." }
    }

    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey(KEY_ALIAS, null) as? SecretKey)?.let { return it }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        generator.init(
            KeyGenParameterSpec.Builder(
                KEY_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
            ).setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        return generator.generateKey()
    }

    private fun preferenceKey(origin: String): String = MessageDigest.getInstance("SHA-256")
        .digest(origin.toByteArray(Charsets.UTF_8))
        .joinToString("") { "%02x".format(it) }

    companion object { private const val KEY_ALIAS = "com.max1874.ferry.device-token" }
}

object FerryCipher {
    fun encrypt(key: SecretKey, plaintext: String): String {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key)
        val encrypted = cipher.doFinal(plaintext.toByteArray(Charsets.UTF_8))
        return Base64.getEncoder().encodeToString(cipher.iv + encrypted)
    }

    fun decrypt(key: SecretKey, encoded: String): String {
        val bytes = Base64.getDecoder().decode(encoded)
        require(bytes.size > IV_BYTES) { "Invalid encrypted credential." }
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, bytes.copyOfRange(0, IV_BYTES)))
        return cipher.doFinal(bytes.copyOfRange(IV_BYTES, bytes.size)).toString(Charsets.UTF_8)
    }

    private const val IV_BYTES = 12
}

interface SettingsStore {
    var origin: String?
    var deviceName: String?
    var clipboardSync: Boolean
}

class AndroidSettingsStore(context: Context) : SettingsStore {
    private val preferences = context.getSharedPreferences("ferry.settings", Context.MODE_PRIVATE)
    override var origin: String?
        get() = preferences.getString("origin", null)
        set(value) { preferences.edit().putString("origin", value).apply() }
    override var deviceName: String?
        get() = preferences.getString("device_name", null)
        set(value) { preferences.edit().putString("device_name", value).apply() }
    override var clipboardSync: Boolean
        get() = preferences.getBoolean("clipboard_sync", false)
        set(value) { preferences.edit().putBoolean("clipboard_sync", value).apply() }
}
