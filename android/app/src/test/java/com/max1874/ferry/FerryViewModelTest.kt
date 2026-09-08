package com.max1874.ferry

import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class FerryViewModelTest {
    @Test fun staleJoinCannotReconnectAfterAddressChanges() = runTest {
        val pending = CompletableDeferred<AccessClaim>()
        val service = FakeService().apply { joinResult = { pending.await() } }
        val model = model(service, this)
        model.updateServerAddress("http://10.0.0.2:42817")
        model.updateDeviceName("Pixel")
        model.connect()
        runCurrent()

        model.updateServerAddress("http://10.0.0.3:42817")
        pending.complete(AccessClaim(device(), "token"))
        runCurrent()

        assertEquals(ConnectionPhase.SETUP, model.state.value.phase)
        assertEquals("http://10.0.0.3:42817", model.state.value.serverAddress)
        assertNull(model.state.value.currentDevice)
    }

    @Test fun authenticatedUnauthorizedResponseRevokesOnlyThatOrigin() = runTest {
        val credentials = MemoryCredentials().apply { values["http://10.0.0.2:42817"] = "expired" }
        val settings = MemorySettings("http://10.0.0.2:42817", "Pixel")
        val service = FakeService().apply {
            sessionResult = { throw FerryApiException(401, "invalid_token", "expired") }
        }
        val dispatcher = StandardTestDispatcher(testScheduler)
        val model = FerryViewModel(
            service, credentials, settings, MemoryContentStore(), "Android", this, dispatcher,
        )

        model.start()
        runCurrent()

        assertEquals(ConnectionPhase.SETUP, model.state.value.phase)
        assertEquals("This device is no longer connected.", model.state.value.statusMessage)
        assertNull(credentials.values["http://10.0.0.2:42817"])
    }

    @Test fun authenticatedUnauthorizedReportsCredentialCleanupFailure() = runTest {
        val credentials = MemoryCredentials().apply {
            values["http://10.0.0.2:42817"] = "expired"
            removeError = IllegalStateException("storage unavailable")
        }
        val service = FakeService().apply {
            sessionResult = { throw FerryApiException(401, "invalid_token", "expired") }
        }
        val dispatcher = StandardTestDispatcher(testScheduler)
        val model = FerryViewModel(
            service, credentials, MemorySettings("http://10.0.0.2:42817", "Pixel"),
            MemoryContentStore(), "Android", this, dispatcher,
        )

        model.start()
        runCurrent()

        assertEquals(ConnectionPhase.SETUP, model.state.value.phase)
        assertEquals(
            "This device is no longer connected. Its saved credential could not be removed: storage unavailable",
            model.state.value.statusMessage,
        )
    }

    @Test fun authenticatedUnauthorizedReportsDestinationCleanupFailure() = runTest {
        val credentials = MemoryCredentials().apply { values["http://10.0.0.2:42817"] = "expired" }
        val service = FakeService().apply {
            sessionResult = {
                throw FerryApiException(401, "invalid_token", "expired", cleanupFailed = true)
            }
        }
        val dispatcher = StandardTestDispatcher(testScheduler)
        val model = FerryViewModel(
            service, credentials, MemorySettings("http://10.0.0.2:42817", "Pixel"),
            MemoryContentStore(), "Android", this, dispatcher,
        )

        model.start()
        runCurrent()

        assertEquals(
            "This device is no longer connected. The incomplete destination file could not be removed.",
            model.state.value.statusMessage,
        )
    }

    @Test fun localSendDoesNotAdvanceTheServerPollCursor() = runTest {
        val service = FakeService().apply {
            joinResult = { AccessClaim(device(), "token") }
            sendTextResult = {
                FerryMessage.Text(
                    "00000000000000000000000000000003", 3, "Pixel", DeviceKind.ANDROID,
                    "2026-08-30T10:00:03Z", it,
                )
            }
        }
        val model = model(service, this)
        model.updateServerAddress("http://10.0.0.2:42817")
        model.updateDeviceName("Pixel")
        model.connect()
        runCurrent()
        model.updateDraft("mine")
        model.send()
        runCurrent()
        advanceTimeBy(2_000)
        runCurrent()

        assertEquals(0, service.messageCursors.last())
        model.disconnect()
    }

    @Test fun laterFileSelectionWinsEvenWhenTheFirstLookupFinishesLast() = runTest {
        val first = CompletableDeferred<SelectedContent>()
        val second = CompletableDeferred<SelectedContent>()
        val content = DeferredContentStore(mapOf("first" to first, "second" to second))
        val model = model(FakeService(), this, content)
        model.selectContent("first")
        model.selectContent("second")
        runCurrent()

        second.complete(SelectedContent("second", "second.txt", "text/plain", 2))
        runCurrent()
        first.complete(SelectedContent("first", "first.txt", "text/plain", 1))
        runCurrent()

        assertEquals("second", model.state.value.selectedFile?.uri)
    }

    @Test fun storedSessionRetriesAfterATemporaryFailure() = runTest {
        var attempts = 0
        val service = FakeService().apply {
            sessionResult = {
                attempts++
                if (attempts == 1) throw FerryApiException(503, "unavailable", "offline")
                device()
            }
        }
        val credentials = MemoryCredentials().apply { values["http://10.0.0.2:42817"] = "token" }
        val dispatcher = StandardTestDispatcher(testScheduler)
        val model = FerryViewModel(
            service, credentials, MemorySettings("http://10.0.0.2:42817", "Pixel"),
            MemoryContentStore(), "Android", this, dispatcher,
        )
        model.start()
        runCurrent()
        assertEquals(ConnectionPhase.OFFLINE, model.state.value.phase)

        advanceTimeBy(2_000)
        runCurrent()

        assertEquals(2, attempts)
        assertEquals(ConnectionPhase.CONNECTED, model.state.value.phase)
        assertEquals("Pixel", model.state.value.currentDevice?.name)
        model.disconnect()
    }

    @Test fun downloadWithoutARestoredSessionDeletesTheEmptyDestination() = runTest {
        val content = MemoryContentStore()
        val model = model(FakeService(), this, content)
        model.download(
            FerryFile("file.txt", "text/plain", 1, "/api/v1/files/0123456789abcdef0123456789abcdef"),
            "0123456789abcdef0123456789abcdef",
            "destination",
        )

        assertEquals(listOf("destination"), content.deleted)
        assertEquals("Reconnect to Ferry, then try saving the file again.", model.state.value.sendError)
    }

    @Test fun disconnectReportsWhenTheSavedCredentialCannotBeRemoved() = runTest {
        val credentials = MemoryCredentials().apply { removeError = IllegalStateException("storage unavailable") }
        val service = FakeService().apply { joinResult = { AccessClaim(device(), "token") } }
        val dispatcher = StandardTestDispatcher(testScheduler)
        val model = FerryViewModel(
            service, credentials, MemorySettings(), MemoryContentStore(), "Android", this, dispatcher,
        )
        model.updateServerAddress("http://10.0.0.2:42817")
        model.updateDeviceName("Pixel")
        model.connect()
        runCurrent()

        model.disconnect()

        assertEquals(ConnectionPhase.SETUP, model.state.value.phase)
        assertEquals(
            "Disconnected for now, but the saved credential could not be removed: storage unavailable",
            model.state.value.statusMessage,
        )
    }

    @Test fun userDownloadContinuesWhileTheActivityIsTemporarilyStopped() = runTest {
        val pending = CompletableDeferred<Unit>()
        val service = FakeService().apply {
            joinResult = { AccessClaim(device(), "token") }
            downloadResult = { pending.await() }
        }
        val model = model(service, this)
        model.updateServerAddress("http://10.0.0.2:42817")
        model.updateDeviceName("Pixel")
        model.connect()
        runCurrent()
        val file = FerryFile("file.txt", "text/plain", 1, "/api/v1/files/0123456789abcdef0123456789abcdef")
        model.download(file, "0123456789abcdef0123456789abcdef", "destination")
        runCurrent()

        model.setActive(false)
        pending.complete(Unit)
        runCurrent()

        assertEquals("Saved file.txt", model.state.value.statusMessage)
        model.disconnect()
    }

    @Test fun clipboardSyncStaysOffUntilTheUserTurnsItOn() = runTest {
        val service = PagedService(listOf(emptyList(), listOf(textMessage(2, "from the study Mac"))))
        val clipboard = RecordingClipboard()
        connected(service, this, clipboard, syncEnabled = false)
        advanceTimeBy(2_100)
        runCurrent()

        assertEquals(emptyList<String>(), clipboard.writes)
    }

    // Connecting replays everything from before the app was running. Writing it
    // would replace what the user was carrying with an entry they never asked for.
    @Test fun connectingDoesNotReplaceTheClipboardWithTheBacklog() = runTest {
        val service = PagedService(listOf(listOf(textMessage(1, "older"), textMessage(2, "newer"))))
        val clipboard = RecordingClipboard()
        connected(service, this, clipboard)
        advanceTimeBy(2_100)
        runCurrent()

        assertEquals(emptyList<String>(), clipboard.writes)
    }

    @Test fun onlyTheNewestMessageFromAnotherDeviceReachesTheClipboard() = runTest {
        val service = PagedService(
            listOf(emptyList(), listOf(textMessage(4, "older"), textMessage(7, "newest"), textMessage(6, "middle"))),
        )
        val clipboard = RecordingClipboard()
        connected(service, this, clipboard)
        advanceTimeBy(2_100)
        runCurrent()

        assertEquals(listOf("newest"), clipboard.writes)
    }

    @Test fun thisDevicesOwnMessagesNeverReachTheClipboard() = runTest {
        val service = PagedService(listOf(emptyList(), listOf(textMessage(3, "sent from here", isCurrentDevice = true))))
        val clipboard = RecordingClipboard()
        connected(service, this, clipboard)
        advanceTimeBy(2_100)
        runCurrent()

        assertEquals(emptyList<String>(), clipboard.writes)
    }

    // Two devices syncing to each other must not hand the same string back and
    // forth for as long as both are open.
    @Test fun textAlreadyOnTheClipboardIsNotWrittenTwice() = runTest {
        val service = PagedService(
            listOf(emptyList(), listOf(textMessage(2, "same")), listOf(textMessage(3, "same"))),
        )
        val clipboard = RecordingClipboard()
        connected(service, this, clipboard)
        advanceTimeBy(4_200)
        runCurrent()

        assertEquals(listOf("same"), clipboard.writes)
    }

    // An image cannot be handed to the Android clipboard without a content
    // provider, so Ferry must leave file messages alone rather than half-do it.
    @Test fun fileMessagesAreLeftOffTheClipboard() = runTest {
        val service = PagedService(listOf(emptyList(), listOf(fileMessage(5, "shot.png", "image/png"))))
        val clipboard = RecordingClipboard()
        connected(service, this, clipboard)
        advanceTimeBy(2_100)
        runCurrent()

        assertEquals(emptyList<String>(), clipboard.writes)
    }

    @Test fun sendingAnEmptyClipboardSendsNothingAndSaysSo() = runTest {
        val service = PagedService(listOf(emptyList()))
        val model = connected(service, this, RecordingClipboard())
        model.sendClipboard("   ")
        runCurrent()

        assertEquals("The clipboard has nothing Ferry can send.", model.state.value.clipboardStatus)
        assertEquals(emptyList<String>(), service.sentTexts)
    }

    @Test fun sendingTheClipboardPostsItsText() = runTest {
        val service = PagedService(listOf(emptyList()))
        val model = connected(service, this, RecordingClipboard())
        model.sendClipboard("carried across")
        runCurrent()

        assertEquals(listOf("carried across"), service.sentTexts)
    }

    @Test fun clipboardPreferenceOutlivesTheSession() = runTest {
        val settings = MemorySettings()
        val dispatcher = StandardTestDispatcher(testScheduler)
        val first = FerryViewModel(PagedService(emptyList()), MemoryCredentials(), settings,
                                   MemoryContentStore(), "Android", this, dispatcher)
        assertEquals(false, first.state.value.clipboardSyncEnabled)
        first.setClipboardSync(true)

        val second = FerryViewModel(PagedService(emptyList()), MemoryCredentials(), settings,
                                    MemoryContentStore(), "Android", this, dispatcher)
        assertEquals(true, second.state.value.clipboardSyncEnabled)
    }

    private fun connected(
        service: FerryService,
        scope: TestScope,
        clipboard: ClipboardWriter,
        syncEnabled: Boolean = true,
    ): FerryViewModel {
        val dispatcher = StandardTestDispatcher(scope.testScheduler)
        val model = FerryViewModel(
            service, MemoryCredentials(), MemorySettings(), MemoryContentStore(), "Android", scope, dispatcher,
            clipboard,
        )
        model.setClipboardSync(syncEnabled)
        model.updateServerAddress("http://10.0.0.2:42817")
        model.updateDeviceName("Pixel")
        model.connect()
        scope.testScheduler.runCurrent()
        return model
    }

    private fun messageId(marker: Char) = marker.toString().repeat(32)

    private fun textMessage(sequence: Long, text: String, isCurrentDevice: Boolean = false) =
        FerryMessage.Text(messageId('a'), sequence, "Study Mac", DeviceKind.MAC,
                          "2026-09-08T00:00:00Z", text, isCurrentDevice)

    private fun fileMessage(sequence: Long, name: String, mediaType: String) =
        FerryMessage.File(messageId('b'), sequence, "Study Mac", DeviceKind.MAC, "2026-09-08T00:00:00Z",
                          FerryFile(name, mediaType, 11, "/api/v1/files/${messageId('b')}"))

    private fun model(
        service: FerryService,
        scope: TestScope,
        contentStore: ContentStore = MemoryContentStore(),
    ): FerryViewModel {
        val dispatcher = StandardTestDispatcher(scope.testScheduler)
        return FerryViewModel(
            service, MemoryCredentials(), MemorySettings(), contentStore, "Android", scope, dispatcher,
        )
    }

    private fun device() = FerryDevice(
        "0123456789abcdef0123456789abcdef", "Pixel", DeviceKind.ANDROID, "2026-08-30T10:00:00Z",
    )

    private class FakeService : FerryService {
        var joinResult: suspend () -> AccessClaim = { error("unexpected join") }
        var sessionResult: suspend () -> FerryDevice = { error("unexpected session") }
        var sendTextResult: suspend (String) -> FerryMessage = { error("unexpected send") }
        var downloadResult: suspend () -> Unit = { }
        val messageCursors = mutableListOf<Long>()
        override suspend fun join(endpoint: ServerEndpoint, deviceName: String, password: String) = joinResult()
        override suspend fun session(endpoint: ServerEndpoint, token: String) = sessionResult()
        override suspend fun messages(endpoint: ServerEndpoint, token: String, after: Long): MessagesPage {
            messageCursors += after
            return MessagesPage(emptyList(), after)
        }
        override suspend fun sendText(endpoint: ServerEndpoint, token: String, text: String) = sendTextResult(text)
        override suspend fun sendFile(endpoint: ServerEndpoint, token: String, file: SelectedContent): FerryMessage = error("unexpected")
        override suspend fun download(endpoint: ServerEndpoint, token: String, file: FerryFile, destinationUri: String) =
            downloadResult()
    }

    /** Serves one scripted page per refresh, then nothing, so a test advances
     *  the timeline by letting the poll interval elapse. */
    private class PagedService(pages: List<List<FerryMessage>>) : FerryService {
        private val remaining = pages.toMutableList()
        val sentTexts = mutableListOf<String>()
        private val device = FerryDevice("d".repeat(32), "Pixel", DeviceKind.ANDROID, "2026-09-08T00:00:00Z")

        override suspend fun join(endpoint: ServerEndpoint, deviceName: String, password: String) =
            AccessClaim(device, "clipboard-token")
        override suspend fun session(endpoint: ServerEndpoint, token: String) = device
        override suspend fun messages(endpoint: ServerEndpoint, token: String, after: Long): MessagesPage {
            if (remaining.isEmpty()) return MessagesPage(emptyList(), after)
            val page = remaining.removeAt(0)
            return MessagesPage(page, page.maxOfOrNull { it.sequence } ?: after)
        }
        override suspend fun sendText(endpoint: ServerEndpoint, token: String, text: String): FerryMessage {
            sentTexts += text
            return FerryMessage.Text("c".repeat(32), 99, "Pixel", DeviceKind.ANDROID,
                                     "2026-09-08T00:00:00Z", text, true)
        }
        override suspend fun sendFile(endpoint: ServerEndpoint, token: String, file: SelectedContent): FerryMessage =
            error("unexpected")
        override suspend fun download(endpoint: ServerEndpoint, token: String, file: FerryFile, destinationUri: String) = Unit
    }

    private class MemoryCredentials : CredentialStore {
        val values = mutableMapOf<String, String>()
        var removeError: Exception? = null
        override fun token(origin: String) = values[origin]
        override fun save(origin: String, token: String) { values[origin] = token }
        override fun remove(origin: String) {
            removeError?.let { throw it }
            values.remove(origin)
        }
    }

    private class MemorySettings(
        override var origin: String? = null,
        override var deviceName: String? = null,
        override var clipboardSync: Boolean = false,
    ) : SettingsStore

    private class RecordingClipboard : ClipboardWriter {
        val writes = mutableListOf<String>()
        override fun write(label: String, text: String) { writes += text }
    }

    private class MemoryContentStore : ContentStore {
        val deleted = mutableListOf<String>()
        override suspend fun describe(uri: String) = SelectedContent(uri, "file", "application/octet-stream", 0)
        override fun open(uri: String) = ByteArrayInputStream(ByteArray(0))
        override fun create(uri: String) = ByteArrayOutputStream()
        override fun delete(uri: String): Boolean { deleted += uri; return true }
    }

    private class DeferredContentStore(
        private val results: Map<String, CompletableDeferred<SelectedContent>>,
    ) : ContentStore {
        override suspend fun describe(uri: String) = results.getValue(uri).await()
        override fun open(uri: String) = ByteArrayInputStream(ByteArray(0))
        override fun create(uri: String) = ByteArrayOutputStream()
        override fun delete(uri: String) = true
    }
}
