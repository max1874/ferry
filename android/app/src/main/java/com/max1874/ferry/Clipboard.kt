package com.max1874.ferry

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context

/**
 * Writing text to the system clipboard, behind a seam so the sync rules can be
 * tested without a real clipboard.
 *
 * Only text. Putting an image on the Android clipboard means publishing it
 * through a content provider and relying on the system to pass a read grant to
 * whichever app pastes it, and that path is not implemented here because it has
 * not been proven on a real device. Sending an image the other way needs none
 * of that and works today.
 *
 * Reading is absent for a different reason: since Android 10 only the focused
 * app or the active keyboard may read the clipboard, and from Android 12 a read
 * shows the user a "pasted from" toast. Ferry therefore reads only when the user
 * presses the send button, never on a timer.
 */
fun interface ClipboardWriter {
    fun write(label: String, text: String)

    companion object {
        /** For hosts with no clipboard to write to, such as unit tests. */
        val None = ClipboardWriter { _, _ -> }
    }
}

class AndroidClipboardWriter(context: Context) : ClipboardWriter {
    private val manager = context.getSystemService(ClipboardManager::class.java)

    override fun write(label: String, text: String) {
        manager?.setPrimaryClip(ClipData.newPlainText(label, text))
    }
}
