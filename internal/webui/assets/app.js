const messagesElement = document.querySelector("#messages");
const welcome = document.querySelector("#welcome");
const conversationElement = document.querySelector("#conversation");
const composerShell = document.querySelector("#composer-shell");
const composer = document.querySelector("#composer");
const textInput = document.querySelector("#text");
const fileInput = document.querySelector("#file");
const sendButton = document.querySelector("#send");
const fileChip = document.querySelector("#file-chip");
const fileName = document.querySelector("#file-name");
const removeFileButton = document.querySelector("#remove-file");
const statusElement = document.querySelector("#status");
const connectionElement = document.querySelector("#connection");
const pairingElement = document.querySelector("#pairing");
const pairingForm = document.querySelector("#pairing-form");
const pairingError = document.querySelector("#pairing-error");
const pairingCodeInput = document.querySelector("#pairing-code");
const deviceNameInput = document.querySelector("#device-name");
const deviceButton = document.querySelector("#device-button");
const devicesDialog = document.querySelector("#devices-dialog");
const closeDevicesButton = document.querySelector("#close-devices");
const currentDeviceElement = document.querySelector("#current-device");
const deviceList = document.querySelector("#device-list");
const generateCodeButton = document.querySelector("#generate-code");
const generatedCode = document.querySelector("#generated-code");

let cursor = 0;
let loading = false;
let sessionLoading = false;
let currentDevice = null;
let deviceToken = readStoredToken();
let authGeneration = 0;
let authController = new AbortController();
let activityStatus = "";
let connectionError = "";
let sendError = "";
let storageError = "";
const rendered = new Set();

function readStoredToken() {
  try {
    return localStorage.getItem("ferry_device_token") || "";
  } catch {
    return "";
  }
}

function storageWritable() {
  try {
    localStorage.setItem("ferry_storage_probe", "1");
    localStorage.removeItem("ferry_storage_probe");
    return true;
  } catch {
    return false;
  }
}

function storeToken(token) {
  try {
    localStorage.setItem("ferry_device_token", token);
    return true;
  } catch {
    return false;
  }
}

function resetTimeline() {
  cursor = 0;
  rendered.clear();
  messagesElement.replaceChildren();
  welcome.hidden = false;
}

function showPairing(message = "", clearCredential = true) {
  authController.abort();
  authController = new AbortController();
  authGeneration += 1;
  currentDevice = null;
  if (clearCredential) {
    deviceToken = "";
    // localStorage is shared by every same-origin tab. A stale tab must not
    // delete a newer token written by another tab; a successful claim replaces it.
    textInput.value = "";
    fileInput.value = "";
    activityStatus = "";
    sendError = "";
    storageError = "";
    resizeComposer();
    updateComposer();
  }
  pairingElement.hidden = false;
  conversationElement.hidden = true;
  composerShell.hidden = true;
  deviceButton.hidden = true;
  connectionElement.textContent = "Pair required";
  pairingError.textContent = message;
  connectionError = "";
  if (devicesDialog.open) devicesDialog.close();
}

function showApp(device) {
  authController.abort();
  authController = new AbortController();
  authGeneration += 1;
  currentDevice = device;
  activityStatus = "";
  connectionError = "";
  pairingElement.hidden = true;
  conversationElement.hidden = false;
  composerShell.hidden = false;
  deviceButton.hidden = false;
  currentDeviceElement.textContent = `Current device: ${device.name}`;
  connectionElement.textContent = "Local";
  pairingError.textContent = "";
  updateComposer();
  renderStatus();
}

function authenticatedFetch(resource, options = {}) {
  const headers = new Headers(options.headers);
  headers.set("Authorization", `Bearer ${deviceToken}`);
  return fetch(resource, { ...options, headers });
}

function setStatus(message, error = false) {
  statusElement.textContent = message;
  statusElement.classList.toggle("error", error);
}

function renderStatus() {
  const error = sendError || storageError || connectionError;
  setStatus(error || activityStatus || "Your data stays on this Ferry server.", Boolean(error));
}

function updateComposer() {
  const selected = fileInput.files.length === 1;
  fileChip.hidden = !selected;
  fileName.textContent = selected ? fileInput.files[0].name : "";
  textInput.disabled = selected;
  sendButton.disabled = selected ? false : textInput.value.trim().length === 0;
}

function resizeComposer() {
  textInput.style.height = "auto";
  textInput.style.height = `${Math.min(textInput.scrollHeight, 180)}px`;
}

function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function renderMessage(message) {
  if (rendered.has(message.id)) return;
  rendered.add(message.id);
  welcome.hidden = true;

  const article = document.createElement("article");
  article.className = "message";
  article.dataset.messageId = message.id;

  const avatar = document.createElement("div");
  avatar.className = "avatar";
  avatar.textContent = message.sender_name.slice(0, 1).toUpperCase();
  avatar.setAttribute("aria-hidden", "true");

  const body = document.createElement("div");
  const head = document.createElement("div");
  head.className = "message-head";
  const sender = document.createElement("span");
  sender.className = "sender";
  sender.textContent = message.sender_name;
  const time = document.createElement("time");
  time.className = "time";
  time.dateTime = message.created_at;
  time.textContent = new Date(message.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  head.append(sender, time);
  body.append(head);

  if (message.kind === "text") {
    const text = document.createElement("p");
    text.className = "message-text";
    text.textContent = message.text;
    body.append(text);
  } else if (message.kind === "file") {
    const link = document.createElement("button");
    link.className = "file-card";
    link.type = "button";
    link.addEventListener("click", (event) => {
      event.preventDefault();
      downloadMessageFile(message);
    });
    const icon = document.createElement("span");
    icon.className = "file-icon";
    icon.textContent = "FILE";
    const copy = document.createElement("span");
    copy.className = "file-copy";
    const title = document.createElement("div");
    title.className = "file-title";
    title.textContent = message.file.name;
    const meta = document.createElement("div");
    meta.className = "file-meta";
    meta.textContent = formatSize(message.file.size);
    copy.append(title, meta);
    link.append(icon, copy);
    body.append(link);
  } else {
    return;
  }

  article.append(avatar, body);
  messagesElement.append(article);
}

async function readError(response) {
  try {
    const body = await response.json();
    return body.error?.message || `Request failed (${response.status})`;
  } catch {
    return `Request failed (${response.status})`;
  }
}

async function downloadMessageFile(message) {
  const generation = authGeneration;
  activityStatus = "Downloading…";
  sendError = "";
  renderStatus();
  try {
    const response = await authenticatedFetch(message.file.download_url, { cache: "no-store", signal: authController.signal });
    if (generation !== authGeneration) return;
    if (response.status === 401) {
      showPairing("This device is no longer paired.");
      return;
    }
    if (!response.ok) throw new Error(await readError(response));
    const blob = await response.blob();
    if (generation !== authGeneration) return;
    const objectURL = URL.createObjectURL(blob);
    const download = document.createElement("a");
    download.href = objectURL;
    download.download = message.file.name;
    document.body.append(download);
    download.click();
    download.remove();
    setTimeout(() => URL.revokeObjectURL(objectURL), 0);
  } catch (error) {
    if (generation !== authGeneration || error.name === "AbortError") return;
    sendError = error.message;
  } finally {
    if (generation === authGeneration) {
      activityStatus = "";
      renderStatus();
    }
  }
}

async function loadMessages() {
  if (loading || !currentDevice) return;
  const generation = authGeneration;
  loading = true;
  const nearBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight - 140;
  try {
    const response = await authenticatedFetch(`/api/v1/messages?after=${cursor}&limit=200`, { cache: "no-store", signal: authController.signal });
    if (generation !== authGeneration) return;
    if (response.status === 401) {
      showPairing("This device is no longer paired.");
      return;
    }
    if (!response.ok) throw new Error(await readError(response));
    const payload = await response.json();
    for (const message of payload.messages) renderMessage(message);
    cursor = payload.next_cursor;
    connectionElement.textContent = "Local";
    connectionError = "";
    renderStatus();
    if (nearBottom && payload.messages.length > 0) window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
  } catch (error) {
    if (generation !== authGeneration || error.name === "AbortError") return;
    connectionElement.textContent = "Offline";
    connectionError = error.message;
    renderStatus();
  } finally {
    loading = false;
  }
}

async function sendText(text) {
  const generation = authGeneration;
  const response = await authenticatedFetch("/api/v1/messages/text", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text }),
    signal: authController.signal,
  });
  if (generation !== authGeneration) return false;
  if (response.status === 401) showPairing("This device is no longer paired.");
  if (!response.ok) throw new Error(await readError(response));
  return true;
}

async function sendFile(file) {
  const generation = authGeneration;
  const form = new FormData();
  form.append("file", file, file.name);
  const response = await authenticatedFetch("/api/v1/messages/file", { method: "POST", body: form, signal: authController.signal });
  if (generation !== authGeneration) return false;
  if (response.status === 401) showPairing("This device is no longer paired.");
  if (!response.ok) throw new Error(await readError(response));
  return true;
}

async function loadSession() {
  if (sessionLoading) return;
  if (!deviceToken) {
    showPairing();
    return;
  }
  const generation = authGeneration;
  const token = deviceToken;
  sessionLoading = true;
  try {
    const response = await authenticatedFetch("/api/v1/session", { cache: "no-store", signal: authController.signal });
    if (generation !== authGeneration || token !== deviceToken) return;
    if (response.status === 401) {
      showPairing();
      return;
    }
    if (!response.ok) throw new Error(await readError(response));
    const payload = await response.json();
    showApp(payload.device);
    await loadMessages();
  } catch (error) {
    if (generation !== authGeneration || token !== deviceToken || error.name === "AbortError") return;
    showPairing(error.message, false);
    connectionElement.textContent = "Offline";
  } finally {
    sessionLoading = false;
  }
}

function pollServer() {
  if (currentDevice) {
    loadMessages();
  } else if (deviceToken) {
    loadSession();
  }
}

pairingForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const submit = pairingForm.querySelector("button[type=submit]");
  submit.disabled = true;
  pairingError.textContent = "";
  try {
    if (!storageWritable()) throw new Error("Browser storage is unavailable. Enable site storage before pairing this device.");
    const response = await fetch("/api/v1/pairing/claim", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ code: pairingCodeInput.value, device_name: deviceNameInput.value }),
    });
    if (!response.ok) throw new Error(await readError(response));
    const payload = await response.json();
    pairingCodeInput.value = "";
    resetTimeline();
    deviceToken = payload.token;
    storageError = storeToken(payload.token) ? "" : "Browser storage is unavailable. Keep this tab open and pair another device before closing it.";
    showApp(payload.device);
    await loadMessages();
  } catch (error) {
    pairingError.textContent = error.message;
  } finally {
    submit.disabled = false;
  }
});

pairingCodeInput.addEventListener("input", () => {
  pairingCodeInput.value = pairingCodeInput.value.toUpperCase();
});

async function loadDevices() {
  const generation = authGeneration;
  const response = await authenticatedFetch("/api/v1/devices", { cache: "no-store", signal: authController.signal });
  if (generation !== authGeneration) return;
  if (response.status === 401) {
    showPairing("This device is no longer paired.");
    return;
  }
  if (!response.ok) throw new Error(await readError(response));
  const payload = await response.json();
  deviceList.replaceChildren();
  for (const device of payload.devices) {
    const row = document.createElement("div");
    row.className = "device-row";
    const copy = document.createElement("div");
    const name = document.createElement("div");
    name.className = "device-name";
    name.textContent = device.name;
    const meta = document.createElement("div");
    meta.className = "device-meta";
    meta.textContent = device.id === currentDevice.id ? "This device" : `Paired ${new Date(device.created_at).toLocaleDateString()}`;
    copy.append(name, meta);
    row.append(copy);
    if (device.id !== currentDevice.id) {
      const revoke = document.createElement("button");
      revoke.className = "revoke";
      revoke.type = "button";
      revoke.textContent = "Revoke";
      revoke.addEventListener("click", async () => {
        revoke.disabled = true;
        const generation = authGeneration;
        try {
          const result = await authenticatedFetch(`/api/v1/devices/${device.id}`, { method: "DELETE", signal: authController.signal });
          if (generation !== authGeneration) return;
          if (result.status === 401) {
            showPairing("This device is no longer paired.");
            return;
          }
          if (!result.ok) throw new Error(await readError(result));
          await loadDevices();
        } catch (error) {
          if (generation !== authGeneration || error.name === "AbortError") return;
          generatedCode.textContent = error.message;
          revoke.disabled = false;
        }
      });
      row.append(revoke);
    }
    deviceList.append(row);
  }
}

deviceButton.addEventListener("click", async () => {
  generatedCode.textContent = "";
  devicesDialog.showModal();
  try {
    await loadDevices();
  } catch (error) {
    if (error.name === "AbortError") return;
    generatedCode.textContent = error.message;
  }
});

closeDevicesButton.addEventListener("click", () => devicesDialog.close());

generateCodeButton.addEventListener("click", async () => {
  generateCodeButton.disabled = true;
  const generation = authGeneration;
  try {
    const response = await authenticatedFetch("/api/v1/pairing/codes", { method: "POST", signal: authController.signal });
    if (generation !== authGeneration) return;
    if (response.status === 401) {
      showPairing("This device is no longer paired.");
      return;
    }
    if (!response.ok) throw new Error(await readError(response));
    const code = await response.json();
    generatedCode.textContent = `${code.code} · expires ${new Date(code.expires_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`;
  } catch (error) {
    if (generation !== authGeneration || error.name === "AbortError") return;
    generatedCode.textContent = error.message;
  } finally {
    generateCodeButton.disabled = false;
  }
});

composer.addEventListener("submit", async (event) => {
  event.preventDefault();
  const selectedFile = fileInput.files[0];
  const text = textInput.value;
  const generation = authGeneration;
  sendButton.disabled = true;
  sendError = "";
  activityStatus = selectedFile ? "Sending file…" : "Sending…";
  renderStatus();
  try {
    if (selectedFile) {
      if (!await sendFile(selectedFile)) return;
      fileInput.value = "";
    } else {
      if (!await sendText(text)) return;
      textInput.value = "";
      resizeComposer();
    }
    activityStatus = "";
    updateComposer();
    await loadMessages();
    window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
    renderStatus();
  } catch (error) {
    if (generation !== authGeneration || error.name === "AbortError") return;
    activityStatus = "";
    sendError = error.message;
    renderStatus();
    updateComposer();
  }
});

textInput.addEventListener("input", () => {
  updateComposer();
  resizeComposer();
});

textInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
    event.preventDefault();
    if (!sendButton.disabled) composer.requestSubmit();
  }
});

fileInput.addEventListener("change", updateComposer);
removeFileButton.addEventListener("click", () => {
  fileInput.value = "";
  updateComposer();
  textInput.focus();
});

updateComposer();
resizeComposer();
loadSession();
window.setInterval(pollServer, 1500);
