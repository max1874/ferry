package com.max1874.ferry

import android.app.Activity
import android.content.Intent
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.safeDrawingPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Switch
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

private val ferryBlue = Color(0xFF0A84FF)
private val darkScheme = darkColorScheme(primary = ferryBlue, background = Color(0xFF202020), surface = Color(0xFF2B2B2B))
private val lightScheme = lightColorScheme(primary = Color(0xFF0066CC), background = Color(0xFFFAFAFA), surface = Color.White)

@Composable
fun FerryApp(model: FerryViewModel) {
    val state by model.state.collectAsStateWithLifecycle()
    var pendingDownloadId by rememberSaveable { mutableStateOf<String?>(null) }
    var pendingDownloadName by rememberSaveable { mutableStateOf<String?>(null) }
    var pendingDownloadType by rememberSaveable { mutableStateOf<String?>(null) }
    var pendingDownloadSize by rememberSaveable { mutableStateOf<Long?>(null) }
    var pendingDownloadUrl by rememberSaveable { mutableStateOf<String?>(null) }
    val photoPicker = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        uri?.let { model.selectContent(it.toString()) }
    }
    val filePicker = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        uri?.let { model.selectContent(it.toString()) }
    }
    val createDocument = rememberLauncherForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        val pendingId = pendingDownloadId
        val pendingName = pendingDownloadName
        val pendingType = pendingDownloadType
        val pendingSize = pendingDownloadSize
        val pendingUrl = pendingDownloadUrl
        pendingDownloadId = null
        pendingDownloadName = null
        pendingDownloadType = null
        pendingDownloadSize = null
        pendingDownloadUrl = null
        if (result.resultCode == Activity.RESULT_OK && pendingId != null && pendingName != null &&
            pendingType != null && pendingSize != null && pendingUrl != null
        ) {
            result.data?.data?.let {
                model.download(FerryFile(pendingName, pendingType, pendingSize, pendingUrl), pendingId, it.toString())
            }
        }
    }
    val dark = isSystemInDarkTheme()
    MaterialTheme(colorScheme = if (dark) darkScheme else lightScheme) {
        Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
            Box(Modifier.fillMaxSize().safeDrawingPadding()) {
                if (state.phase == ConnectionPhase.SETUP || state.phase == ConnectionPhase.CONNECTING) {
                    SetupScreen(state, model)
                } else {
                    TimelineScreen(
                        state = state,
                        model = model,
                        onPickPhoto = {
                            photoPicker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageAndVideo))
                        },
                        onPickFile = { filePicker.launch(arrayOf("*/*")) },
                        onDownload = { id, file ->
                            pendingDownloadId = id
                            pendingDownloadName = file.name
                            pendingDownloadType = file.mediaType
                            pendingDownloadSize = file.size
                            pendingDownloadUrl = file.downloadUrl
                            val intent = Intent(Intent.ACTION_CREATE_DOCUMENT).apply {
                                addCategory(Intent.CATEGORY_OPENABLE)
                                type = file.mediaType.ifBlank { "application/octet-stream" }
                                putExtra(Intent.EXTRA_TITLE, file.name)
                            }
                            createDocument.launch(intent)
                        },
                    )
                }
            }
        }
    }
}

@Composable
private fun SetupScreen(state: FerryUiState, model: FerryViewModel) {
    Column(
        modifier = Modifier.fillMaxSize().imePadding().verticalScroll(rememberScrollState()).padding(horizontal = 24.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        Surface(shape = RoundedCornerShape(22.dp), color = Color(0xFF071421), modifier = Modifier.size(72.dp)) {
            Box(contentAlignment = Alignment.Center) { Text("F", color = Color.White, fontSize = 32.sp, fontWeight = FontWeight.Bold) }
        }
        Spacer(Modifier.height(24.dp))
        Text("Connect to Ferry", style = MaterialTheme.typography.headlineLarge, fontWeight = FontWeight.Bold)
        Text("Your clipboard and files, across your own devices.", color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(28.dp))
        OutlinedTextField(
            value = state.serverAddress,
            onValueChange = model::updateServerAddress,
            label = { Text("Server address") },
            placeholder = { Text("http://10.0.0.2:42817") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(12.dp))
        OutlinedTextField(
            value = state.deviceName,
            onValueChange = model::updateDeviceName,
            label = { Text("Device name") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(12.dp))
        OutlinedTextField(
            value = state.password,
            onValueChange = model::updatePassword,
            label = { Text("Password (if enabled)") },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
            visualTransformation = PasswordVisualTransformation(),
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        if (state.serverAddress.trim().startsWith("http://")) {
            Text(
                "Unencrypted HTTP — use only on your trusted local network.",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = 10.dp),
            )
        }
        state.statusMessage?.let { ErrorText(it) }
        Spacer(Modifier.height(18.dp))
        Button(
            onClick = model::connect,
            enabled = state.phase != ConnectionPhase.CONNECTING,
            modifier = Modifier.fillMaxWidth().height(52.dp),
        ) {
            if (state.phase == ConnectionPhase.CONNECTING) {
                CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                Spacer(Modifier.width(10.dp))
            }
            Text("Connect")
        }
    }
}

@Composable
private fun TimelineScreen(
    state: FerryUiState,
    model: FerryViewModel,
    onPickPhoto: () -> Unit,
    onPickFile: () -> Unit,
    onDownload: (String, FerryFile) -> Unit,
) {
    val listState = rememberLazyListState()
    LaunchedEffect(state.messages.size) {
        if (state.messages.isNotEmpty()) listState.animateScrollToItem(state.messages.lastIndex)
    }
    Column(Modifier.fillMaxSize().imePadding()) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 18.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text("Ferry", style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold)
            Spacer(Modifier.weight(1f))
            Text(state.currentDevice?.name ?: "Android", style = MaterialTheme.typography.labelMedium)
            Spacer(Modifier.width(8.dp))
            Box {
                var settingsOpen by remember { mutableStateOf(false) }
                OutlinedButton(onClick = { settingsOpen = true }) { Text("Settings") }
                DropdownMenu(expanded = settingsOpen, onDismissRequest = { settingsOpen = false }) {
                    DropdownMenuItem(
                        text = { Text(if (state.clipboardSyncEnabled) "Clipboard sync on" else "Clipboard sync off") },
                        onClick = { model.setClipboardSync(!state.clipboardSyncEnabled) },
                        trailingIcon = {
                            Switch(checked = state.clipboardSyncEnabled, onCheckedChange = { model.setClipboardSync(it) })
                        },
                    )
                    DropdownMenuItem(
                        text = { Text("Change server") },
                        onClick = { settingsOpen = false; model.disconnect() },
                    )
                }
            }
        }
        if (state.messages.isEmpty()) {
            Box(Modifier.weight(1f).fillMaxWidth(), contentAlignment = Alignment.Center) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text("Ready to ferry", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
                    Text("Send text or a file to your other devices.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        } else {
            LazyColumn(
                state = listState,
                modifier = Modifier.weight(1f).fillMaxWidth(),
                contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 18.dp, vertical = 12.dp),
                verticalArrangement = Arrangement.spacedBy(20.dp),
            ) {
                items(state.messages, key = { it.id }) { message ->
                    MessageRow(message, state.downloadingMessageId == message.id,
                        state.downloadingMessageId != null, onDownload)
                }
            }
        }
        Composer(state, model, onPickPhoto, onPickFile)
    }
}

@Composable
private fun MessageRow(message: FerryMessage, downloading: Boolean, downloadBusy: Boolean,
                       onDownload: (String, FerryFile) -> Unit) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.Top) {
        Surface(shape = RoundedCornerShape(10.dp), color = Color(0xFF111111), modifier = Modifier.size(34.dp)) {
            Box(contentAlignment = Alignment.Center) {
                DeviceIcon(message.senderKind, Color.White, size = 19.dp)
            }
        }
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(message.senderName, fontWeight = FontWeight.Bold)
                Spacer(Modifier.width(8.dp))
                Text(formatTime(message.createdAt), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Spacer(Modifier.height(5.dp))
            when (message) {
                is FerryMessage.Text -> CopyableText(message.text)
                is FerryMessage.File -> Surface(
                    shape = RoundedCornerShape(14.dp),
                    tonalElevation = 2.dp,
                    modifier = Modifier.fillMaxWidth().clickable(enabled = !downloadBusy) { onDownload(message.id, message.file) },
                ) {
                    Row(Modifier.padding(14.dp), verticalAlignment = Alignment.CenterVertically) {
                        Surface(shape = RoundedCornerShape(9.dp), color = ferryBlue, modifier = Modifier.size(42.dp)) {
                            Box(contentAlignment = Alignment.Center) { Text("FILE", color = Color.White, fontSize = 10.sp, fontWeight = FontWeight.Bold) }
                        }
                        Spacer(Modifier.width(12.dp))
                        Column(Modifier.weight(1f)) {
                            Text(message.file.name, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            Text(byteCount(message.file.size), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Text(if (downloading) "Saving…" else "Save", color = MaterialTheme.colorScheme.primary)
                    }
                }
            }
        }
    }
}

@Composable
private fun CopyableText(value: String) {
    val clipboard = LocalClipboardManager.current
    SelectionContainer {
        Text(
            value,
            modifier = Modifier.fillMaxWidth().clickable { clipboard.setText(AnnotatedString(value)) }
                .semantics { contentDescription = "Message text. Tap to copy." },
        )
    }
}

@Composable
private fun Composer(state: FerryUiState, model: FerryViewModel, onPickPhoto: () -> Unit, onPickFile: () -> Unit) {
    var menuOpen by remember { mutableStateOf(false) }
    val clipboard = LocalClipboardManager.current
    Column(Modifier.fillMaxWidth().padding(horizontal = 10.dp, vertical = 8.dp)) {
        state.clipboardStatus?.let {
            Text(it, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                 modifier = Modifier.padding(top = 8.dp))
        }
        state.credentialWarning?.let { ErrorText(it) }
        state.statusMessage?.let {
            Text(it, color = if (state.phase == ConnectionPhase.OFFLINE) Color(0xFFFFA726) else MaterialTheme.colorScheme.primary)
        }
        state.sendError?.let { ErrorText(it) }
        state.selectedFile?.let { file ->
            Surface(shape = RoundedCornerShape(14.dp), tonalElevation = 2.dp, modifier = Modifier.fillMaxWidth().padding(bottom = 8.dp)) {
                Row(Modifier.padding(12.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(file.name, modifier = Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(file.size?.let(::byteCount) ?: "Unknown size", style = MaterialTheme.typography.bodySmall)
                    TextButton(onClick = model::clearSelectedFile) { Text("Remove") }
                }
            }
        }
        Surface(shape = RoundedCornerShape(28.dp), tonalElevation = 4.dp) {
            Row(Modifier.fillMaxWidth().padding(8.dp), verticalAlignment = Alignment.Bottom) {
                Box {
                    TextButton(onClick = { menuOpen = true }, modifier = Modifier.size(48.dp)) { Text("+", fontSize = 26.sp) }
                    DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                        DropdownMenuItem(text = { Text("Photos") }, onClick = { menuOpen = false; onPickPhoto() })
                        DropdownMenuItem(text = { Text("Files") }, onClick = { menuOpen = false; onPickFile() })
                    }
                }
                // Android only lets the focused app read the clipboard, and from
                // Android 12 tells the user when it does, so this reads on the
                // press and never on a timer.
                TextButton(
                    onClick = { model.sendClipboard(clipboard.getText()?.text) },
                    enabled = !state.isSending,
                    modifier = Modifier.size(48.dp),
                ) { Text("⎘", fontSize = 20.sp) }
                OutlinedTextField(
                    value = state.draft,
                    onValueChange = model::updateDraft,
                    placeholder = { Text("Message Ferry") },
                    enabled = state.selectedFile == null,
                    minLines = 1,
                    maxLines = 5,
                    modifier = Modifier.weight(1f),
                )
                Spacer(Modifier.width(8.dp))
                Button(
                    onClick = model::send,
                    enabled = state.canSend,
                    shape = CircleShape,
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(0.dp),
                    modifier = Modifier.size(48.dp),
                    colors = ButtonDefaults.buttonColors(),
                ) { Text("↑", fontSize = 22.sp) }
            }
        }
        Text(
            "Your data stays on this Ferry server.",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.align(Alignment.CenterHorizontally).padding(top = 7.dp),
        )
    }
}

@Composable
private fun ErrorText(value: String) {
    Text(value, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(top = 8.dp))
}

private fun formatTime(value: String): String = runCatching {
    DateTimeFormatter.ofPattern("HH:mm").withZone(ZoneId.systemDefault()).format(Instant.parse(value))
}.getOrDefault("")

private fun byteCount(value: Long): String {
    val units = arrayOf("B", "KB", "MB", "GB")
    var amount = value.toDouble()
    var unit = 0
    while (amount >= 1024 && unit < units.lastIndex) {
        amount /= 1024
        unit += 1
    }
    return if (unit == 0) "$value B" else String.format(java.util.Locale.ROOT, "%.1f %s", amount, units[unit])
}
