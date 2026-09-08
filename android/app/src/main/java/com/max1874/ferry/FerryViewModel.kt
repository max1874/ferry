package com.max1874.ferry

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.coroutines.yield

enum class ConnectionPhase { SETUP, CONNECTING, CONNECTED, OFFLINE }

data class FerryUiState(
    val serverAddress: String = "",
    val deviceName: String,
    val password: String = "",
    val draft: String = "",
    val phase: ConnectionPhase = ConnectionPhase.SETUP,
    val currentDevice: FerryDevice? = null,
    val messages: List<FerryMessage> = emptyList(),
    val selectedFile: SelectedContent? = null,
    val isSending: Boolean = false,
    val downloadingMessageId: String? = null,
    val statusMessage: String? = null,
    val sendError: String? = null,
    val credentialWarning: String? = null,
    val clipboardSyncEnabled: Boolean = false,
    val clipboardStatus: String? = null,
) {
    val canSend: Boolean
        get() = !isSending && phase != ConnectionPhase.SETUP &&
            (selectedFile != null || draft.any { !it.isWhitespace() })
}

class FerryViewModel(
    private val service: FerryService,
    private val credentials: CredentialStore,
    private val settings: SettingsStore,
    private val contentStore: ContentStore,
    defaultDeviceName: String,
    private val externalScope: CoroutineScope? = null,
    private val ioDispatcher: CoroutineDispatcher = Dispatchers.IO,
    private val clipboard: ClipboardWriter = ClipboardWriter.None,
) : ViewModel() {
    private val mutableState = MutableStateFlow(
        FerryUiState(
            serverAddress = settings.origin.orEmpty(),
            deviceName = settings.deviceName?.takeIf { it.isNotBlank() } ?: defaultDeviceName,
            clipboardSyncEnabled = settings.clipboardSync,
        ),
    )
    val state: StateFlow<FerryUiState> = mutableState.asStateFlow()

    private val scope: CoroutineScope get() = externalScope ?: viewModelScope
    private var endpoint: ServerEndpoint? = null
    private var token: String? = null
    private var cursor = 0L
    private var generation = 0L
    private var draftRevision = 0L
    private var fileRevision = 0L
    private var sessionJob: Job? = null
    private var restoreJob: Job? = null
    private var pollJob: Job? = null
    private var sendJob: Job? = null
    private var downloadJob: Job? = null
    private var active = true
    private var started = false

    // What Ferry last put on this clipboard. An incoming message that already
    // matches it is not written again, which stops two synced devices from
    // handing the same text back and forth.
    private var clipboardEcho: String? = null

    // The page that arrives on connecting is everything from before the app was
    // running. Writing it would replace what the user was carrying with an entry
    // they never asked for, so it only sets the baseline.
    private var clipboardPrimed = false

    fun start() {
        if (started) return
        started = true
        val origin = settings.origin ?: return
        val parsed = runCatching { ServerEndpoint.parse(origin) }.getOrElse {
            mutableState.update { state -> state.copy(statusMessage = it.message, phase = ConnectionPhase.SETUP) }
            return
        }
        val storedToken = try {
            credentials.token(parsed.origin)
        } catch (error: Exception) {
            mutableState.update { it.copy(statusMessage = readable(error), phase = ConnectionPhase.SETUP) }
            return
        } ?: return
        resetSession(parsed, storedToken)
        mutableState.update { it.copy(phase = ConnectionPhase.CONNECTING, statusMessage = null) }
        startRestoring()
    }

    fun updateServerAddress(value: String) {
        if (value == mutableState.value.serverAddress) return
        if (mutableState.value.phase == ConnectionPhase.CONNECTING) invalidateSession()
        mutableState.update { it.copy(serverAddress = value, phase = ConnectionPhase.SETUP, statusMessage = null) }
    }

    fun updateDeviceName(value: String) = mutableState.update { it.copy(deviceName = value, statusMessage = null) }
    fun updatePassword(value: String) = mutableState.update { it.copy(password = value, statusMessage = null) }
    fun updateDraft(value: String) {
        draftRevision++
        mutableState.update { it.copy(draft = value, sendError = null) }
    }

    fun connect() {
        val snapshot = mutableState.value
        val parsed = try {
            ServerEndpoint.parse(snapshot.serverAddress)
        } catch (error: Exception) {
            mutableState.update { it.copy(statusMessage = readable(error)) }
            return
        }
        val name = try {
            normalizedDeviceName(snapshot.deviceName)
        } catch (error: Exception) {
            mutableState.update { it.copy(statusMessage = readable(error)) }
            return
        }
        val current = resetSession(parsed, null)
        mutableState.update {
            it.copy(
                serverAddress = parsed.origin,
                deviceName = name,
                phase = ConnectionPhase.CONNECTING,
                statusMessage = null,
            )
        }
        sessionJob = scope.launch {
            try {
                val claim = service.join(parsed, name, snapshot.password)
                if (current != generation) return@launch
                endpoint = parsed
                token = claim.token
                settings.origin = parsed.origin
                settings.deviceName = name
                val warning = runCatching { credentials.save(parsed.origin, claim.token) }.exceptionOrNull()?.let(::readable)
                mutableState.update {
                    it.copy(
                        password = "",
                        phase = ConnectionPhase.CONNECTED,
                        currentDevice = claim.device,
                        credentialWarning = warning,
                    )
                }
                refresh(current)
                startPolling()
            } catch (error: Exception) {
                handle(error, current, authenticated = false)
            }
        }
    }

    fun selectContent(uri: String) {
        val current = generation
        val selection = ++fileRevision
        scope.launch {
            try {
                val file = withContext(ioDispatcher) { contentStore.describe(uri) }
                if (current != generation || selection != fileRevision) return@launch
                if (file.size != null && (file.size < 0 || file.size > FerryJson.MAX_FILE_BYTES)) {
                    throw FerryApiException(413, "payload_too_large", "Files must be 64 MB or smaller.")
                }
                mutableState.update { it.copy(selectedFile = file, sendError = null) }
            } catch (error: Exception) {
                if (current == generation && selection == fileRevision) {
                    mutableState.update { it.copy(sendError = readable(error)) }
                }
            }
        }
    }

    fun clearSelectedFile() {
        fileRevision++
        mutableState.update { it.copy(selectedFile = null) }
    }

    fun send() {
        val parsed = endpoint ?: return
        val credential = token ?: return
        val snapshot = mutableState.value
        if (!snapshot.canSend) return
        val current = generation
        val sentDraftRevision = draftRevision
        val sentFileRevision = fileRevision
        mutableState.update { it.copy(isSending = true, sendError = null) }
        sendJob?.cancel()
        sendJob = scope.launch {
            try {
                val message = snapshot.selectedFile?.let { service.sendFile(parsed, credential, it) }
                    ?: service.sendText(parsed, credential, snapshot.draft)
                if (current != generation) return@launch
                append(message)
                mutableState.update {
                    it.copy(
                        draft = if (sentDraftRevision == draftRevision && snapshot.selectedFile == null) "" else it.draft,
                        selectedFile = if (sentFileRevision == fileRevision && snapshot.selectedFile != null) null else it.selectedFile,
                    )
                }
                if (sentDraftRevision == draftRevision && snapshot.selectedFile == null) draftRevision++
                if (sentFileRevision == fileRevision && snapshot.selectedFile != null) fileRevision++
            } catch (_: CancellationException) {
                return@launch
            } catch (error: Exception) {
                if (current == generation) {
                    if (isUnauthorized(error)) handle(error, current, authenticated = true)
                    else mutableState.update { it.copy(sendError = readable(error)) }
                }
            } finally {
                if (current == generation) mutableState.update { it.copy(isSending = false) }
            }
        }
    }

    fun download(file: FerryFile, messageId: String, destinationUri: String) {
        val parsed = endpoint
        val credential = token
        if (parsed == null || credential == null) {
            val removed = contentStore.delete(destinationUri)
            mutableState.update {
                it.copy(
                    sendError = if (removed) "Reconnect to Ferry, then try saving the file again."
                    else "Reconnect to Ferry and remove the empty destination file before trying again.",
                )
            }
            return
        }
        if (downloadJob?.isActive == true) {
            val removed = contentStore.delete(destinationUri)
            mutableState.update {
                it.copy(
                    sendError = if (removed) "Wait for the current download to finish."
                    else "Wait for the current download and remove the empty destination file.",
                )
            }
            return
        }
        val current = generation
        mutableState.update { it.copy(downloadingMessageId = messageId, sendError = null) }
        downloadJob = scope.launch {
            try {
                service.download(parsed, credential, file, destinationUri)
                if (current == generation) mutableState.update { it.copy(statusMessage = "Saved ${file.name}") }
            } catch (_: CancellationException) {
                return@launch
            } catch (error: Exception) {
                if (current == generation) {
                    if (isUnauthorized(error)) handle(error, current, authenticated = true)
                    else mutableState.update { it.copy(sendError = readable(error)) }
                }
            } finally {
                if (current == generation) mutableState.update { it.copy(downloadingMessageId = null) }
            }
        }
    }

    fun disconnect() {
        val removalError = endpoint?.let {
            runCatching { credentials.remove(it.origin) }.exceptionOrNull()
        }
        invalidateSession()
        mutableState.update {
            FerryUiState(
                serverAddress = it.serverAddress,
                deviceName = it.deviceName,
                // Disconnecting drops the session, not the user's settings. A
                // stored preference the UI reports as off is a lie about what
                // the next connection will do.
                clipboardSyncEnabled = it.clipboardSyncEnabled,
                statusMessage = removalError?.let {
                    "Disconnected for now, but the saved credential could not be removed: ${readable(it)}"
                },
            )
        }
    }

    fun setActive(value: Boolean) {
        active = value
        if (value) {
            if (token != null && mutableState.value.currentDevice == null) startRestoring()
            else if (mutableState.value.phase in setOf(ConnectionPhase.CONNECTED, ConnectionPhase.OFFLINE)) startPolling()
        } else {
            restoreJob?.cancel(); restoreJob = null
            pollJob?.cancel(); pollJob = null
        }
    }

    private fun resetSession(newEndpoint: ServerEndpoint, newToken: String?): Long {
        invalidateSession()
        endpoint = newEndpoint
        token = newToken
        mutableState.update {
            it.copy(
                currentDevice = null,
                messages = emptyList(),
                selectedFile = null,
                draft = "",
                isSending = false,
                downloadingMessageId = null,
                sendError = null,
                credentialWarning = null,
            )
        }
        draftRevision++
        fileRevision++
        return generation
    }

    private fun invalidateSession() {
        generation++
        sessionJob?.cancel(); sessionJob = null
        restoreJob?.cancel(); restoreJob = null
        pollJob?.cancel(); pollJob = null
        sendJob?.cancel(); sendJob = null
        downloadJob?.cancel(); downloadJob = null
        endpoint = null
        token = null
        cursor = 0
        // The next page is a full backfill again, so it must not reach the
        // clipboard, and a new session's echo is nobody's.
        clipboardPrimed = false
        clipboardEcho = null
    }

    private fun startPolling() {
        if (!active || pollJob != null || endpoint == null || token == null) return
        val current = generation
        pollJob = scope.launch {
            while (current == generation) {
                if (refresh(current)) yield() else delay(2_000)
            }
        }
    }

    private fun startRestoring() {
        if (!active || restoreJob != null || endpoint == null || token == null) return
        val current = generation
        val job = scope.launch(start = CoroutineStart.LAZY) {
            try {
                while (current == generation && active) {
                    val parsed = endpoint ?: return@launch
                    val credential = token ?: return@launch
                    try {
                        val device = service.session(parsed, credential)
                        if (current != generation) return@launch
                        mutableState.update { it.copy(phase = ConnectionPhase.CONNECTED, currentDevice = device, statusMessage = null) }
                        refresh(current)
                        startPolling()
                        return@launch
                    } catch (_: CancellationException) {
                        return@launch
                    } catch (error: Exception) {
                        handle(error, current, authenticated = true)
                        if (current != generation || isUnauthorized(error)) return@launch
                        delay(2_000)
                    }
                }
            } finally {
                if (restoreJob === currentCoroutineContext()[Job]) restoreJob = null
            }
        }
        restoreJob = job
        job.start()
    }

    private suspend fun refresh(current: Long): Boolean {
        val parsed = endpoint ?: return false
        val credential = token ?: return false
        try {
            val page = service.messages(parsed, credential, cursor)
            if (current != generation) return false
            val previousCursor = cursor
            page.messages.forEach(::append)
            cursor = maxOf(cursor, page.nextCursor)
            mutableState.update { it.copy(phase = ConnectionPhase.CONNECTED, statusMessage = null) }
            syncClipboard(page.messages, current)
            return page.messages.size == FerryJson.MESSAGE_PAGE_LIMIT && cursor > previousCursor
        } catch (_: CancellationException) {
            return false
        } catch (error: Exception) {
            handle(error, current, authenticated = true)
            return false
        }
    }

    fun setClipboardSync(enabled: Boolean) {
        settings.clipboardSync = enabled
        mutableState.update { it.copy(clipboardSyncEnabled = enabled, clipboardStatus = null) }
    }

    /**
     * Puts the newest message another device sent on this clipboard.
     *
     * Only the newest one: returning to an app that missed twenty messages must
     * leave one clipboard entry, not replay twenty. Only in the foreground,
     * because replacing the clipboard of the app the user is working in is not
     * Ferry's to do — and since Android 10 a background app cannot read the
     * clipboard anyway, so nothing here can be made to work from behind.
     *
     * Images are left alone. Handing an image to the Android clipboard means
     * publishing it through a content provider and trusting the system to pass
     * a read grant to whichever app pastes it; that has not been proven on a
     * real device, so Ferry does not claim it.
     */
    private fun syncClipboard(messages: List<FerryMessage>, current: Long) {
        if (!state.value.clipboardSyncEnabled || !active || current != generation) return
        if (!clipboardPrimed) {
            clipboardPrimed = true
            return
        }
        val latest = messages.filterNot { it.isCurrentDevice }.maxByOrNull { it.sequence } ?: return
        if (latest !is FerryMessage.Text) return
        if (latest.text == clipboardEcho) return
        clipboard.write("Ferry", latest.text)
        clipboardEcho = latest.text
        mutableState.update { it.copy(clipboardStatus = "Copied the newest message.") }
    }

    /**
     * Sends what the user handed over from the clipboard. Android only lets the
     * focused app read the clipboard and tells the user when it does, so this
     * runs from a button press and never on a timer.
     */
    fun sendClipboard(text: String?) {
        val content = text?.trim()
        if (content.isNullOrEmpty()) {
            mutableState.update { it.copy(clipboardStatus = "The clipboard has nothing Ferry can send.") }
            return
        }
        clearSelectedFile()
        updateDraft(text)
        mutableState.update { it.copy(clipboardStatus = null) }
        // The text is on this device's clipboard either way, so recording it now
        // is right whether or not the send succeeds: it must not be written back
        // when the same string returns from another device.
        clipboardEcho = text
        send()
    }

    private fun append(message: FerryMessage) {
        mutableState.update { state ->
            if (state.messages.any { it.id == message.id }) state
            else state.copy(messages = (state.messages + message).sortedBy { it.sequence })
        }
    }

    private fun handle(error: Exception, current: Long, authenticated: Boolean) {
        if (current != generation || error is CancellationException) return
        if (authenticated && isUnauthorized(error)) {
            val oldEndpoint = endpoint
            invalidateSession()
            val removalError = oldEndpoint?.let {
                runCatching { credentials.remove(it.origin) }.exceptionOrNull()
            }
            val destinationWarning = if ((error as FerryApiException).cleanupFailed)
                " The incomplete destination file could not be removed." else ""
            mutableState.update {
                it.copy(
                    phase = ConnectionPhase.SETUP,
                    currentDevice = null,
                    messages = emptyList(),
                    statusMessage = "This device is no longer connected." + destinationWarning +
                        (removalError?.let { " Its saved credential could not be removed: ${readable(it)}" } ?: ""),
                )
            }
            return
        }
        mutableState.update {
            it.copy(
                phase = if (authenticated) ConnectionPhase.OFFLINE else ConnectionPhase.SETUP,
                statusMessage = readable(error),
            )
        }
    }

    private fun isUnauthorized(error: Exception): Boolean = error is FerryApiException && error.status == 401
    private fun readable(error: Throwable): String = error.message?.takeIf { it.isNotBlank() } ?: "Ferry request failed."

    private fun normalizedDeviceName(value: String): String {
        val name = value.trim()
        if (name.isEmpty() || name.toByteArray(Charsets.UTF_8).size > 64 || name.any(Char::isISOControl)) {
            throw IllegalArgumentException("Device name must be 1–64 UTF-8 bytes without control characters.")
        }
        return name
    }
}
