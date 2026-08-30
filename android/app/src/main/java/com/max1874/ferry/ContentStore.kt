package com.max1874.ferry
import android.content.ContentResolver
import android.net.Uri
import android.provider.DocumentsContract
import android.provider.OpenableColumns
import java.io.InputStream
import java.io.OutputStream
data class SelectedContent(val uri: String, val name: String, val mediaType: String, val size: Long?)

interface ContentStore {
    suspend fun describe(uri: String): SelectedContent
    fun open(uri: String): InputStream
    fun create(uri: String): OutputStream
    fun delete(uri: String): Boolean
}

class AndroidContentStore(private val resolver: ContentResolver) : ContentStore {
    override suspend fun describe(uri: String): SelectedContent {
        val value = Uri.parse(uri)
        var name: String? = null
        var size: Long? = null
        resolver.query(value, arrayOf(OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE), null, null, null)?.use { cursor ->
            if (cursor.moveToFirst()) {
                val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                val sizeIndex = cursor.getColumnIndex(OpenableColumns.SIZE)
                if (nameIndex >= 0 && !cursor.isNull(nameIndex)) name = cursor.getString(nameIndex)
                if (sizeIndex >= 0 && !cursor.isNull(sizeIndex)) size = cursor.getLong(sizeIndex)
            }
        }
        val displayName = name?.takeIf { it.isNotBlank() } ?: value.lastPathSegment ?: "file"
        val mediaType = resolver.getType(value)?.takeIf(::safeMediaType) ?: "application/octet-stream"
        return SelectedContent(uri, displayName, mediaType, size)
    }

    override fun open(uri: String): InputStream = resolver.openInputStream(Uri.parse(uri))
        ?: throw IllegalArgumentException("The selected file cannot be opened.")

    override fun create(uri: String): OutputStream = resolver.openOutputStream(Uri.parse(uri), "wt")
        ?: throw IllegalArgumentException("The destination file cannot be opened.")

    override fun delete(uri: String): Boolean = runCatching {
        DocumentsContract.deleteDocument(resolver, Uri.parse(uri))
    }.getOrDefault(false)

    private fun safeMediaType(value: String): Boolean = value.length <= 127 &&
        value.count { it == '/' } == 1 && value.none { it <= ' ' || it == '\u007f' }
}
