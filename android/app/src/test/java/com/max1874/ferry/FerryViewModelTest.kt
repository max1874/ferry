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
    ) : SettingsStore

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
