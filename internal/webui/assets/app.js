const messagesElement = document.querySelector("#messages");
const sidebar = document.querySelector("#sidebar");
const welcome = document.querySelector("#welcome");
const conversationElement = document.querySelector("#conversation");
const composerShell = document.querySelector("#composer-shell");
const composer = document.querySelector("#composer");
const textInput = document.querySelector("#text");
const photoInput = document.querySelector("#photo");
const fileInput = document.querySelector("#file");
const attachmentPicker = document.querySelector("#attachment-picker");
const attachButton = document.querySelector("#attach");
const attachmentMenu = document.querySelector("#attachment-menu");
const choosePhotosButton = document.querySelector("#choose-photos");
const chooseFilesButton = document.querySelector("#choose-files");
const sendButton = document.querySelector("#send");
const fileChip = document.querySelector("#file-chip");
const fileName = document.querySelector("#file-name");
const removeFileButton = document.querySelector("#remove-file");
const statusElement = document.querySelector("#status");
const connectionElement = document.querySelector("#connection");
const accessElement = document.querySelector("#access");
const accessForm = document.querySelector("#access-form");
const accessError = document.querySelector("#access-error");
const passwordField = document.querySelector("#password-field");
const accessPasswordInput = document.querySelector("#access-password");
const deviceNameInput = document.querySelector("#device-name");
const deviceButton = document.querySelector("#device-button");
const devicesDialog = document.querySelector("#devices-dialog");
const closeDevicesButton = document.querySelector("#close-devices");
const currentDeviceElement = document.querySelector("#current-device");
const deviceList = document.querySelector("#device-list");
const settingsPasswordInput = document.querySelector("#settings-password");
const saveAccessButton = document.querySelector("#save-access");
const accessSettingsStatus = document.querySelector("#access-settings-status");

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

const DEVICE_ICON_PATHS = Object.freeze({
  iphone: ["M6 5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2V5", "M11 4h2", "M12 17v.01"],
  ipad: ["M5 4a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v16a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4", "M11 17a1 1 0 1 0 2 0 1 1 0 0 0-2 0"],
  mac: ["M3 5a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V5", "M7 20h10", "M9 16v4", "M15 16v4"],
  android: ["M4 10v6", "M20 10v6", "M7 9h10v8a1 1 0 0 1-1 1H8a1 1 0 0 1-1-1V9a5 5 0 0 1 10 0", "M8 3l1 2", "M16 3l-1 2", "M9 18v3", "M15 18v3"],
  windows: ["M17.8 20l-12-1.5A2 2 0 0 1 4 16.6V7.4a2 2 0 0 1 1.8-1.9l12-1.5A2 2 0 0 1 20 5.9V18a2 2 0 0 1-2.2 1.9V20", "M12 5v14", "M4 12h16"],
  browser: ["M4 8h16", "M4 6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6", "M8 4v4"],
});

const localDeviceKind = defaultDeviceKind();
deviceNameInput.value = defaultDeviceName(localDeviceKind);

function defaultDeviceKind() {
  const platform = navigator.userAgentData?.platform || navigator.platform || "";
  const userAgent = navigator.userAgent || "";
  if (/ipad/i.test(platform) || /ipad/i.test(userAgent) || /mac/i.test(platform) && navigator.maxTouchPoints > 1) return "ipad";
  if (/iphone|ipod/i.test(platform) || /iphone|ipod/i.test(userAgent)) return "iphone";
  if (/android/i.test(platform) || /android/i.test(userAgent)) return "android";
  if (/mac/i.test(platform)) return "mac";
  if (/win/i.test(platform)) return "windows";
  return "browser";
}

function defaultDeviceName(kind) {
  return { iphone: "iPhone Web", ipad: "iPad Web", mac: "Mac Web", android: "Android Web", windows: "Windows Web", browser: "Web Browser" }[kind];
}

function createDeviceIcon(kind) {
  const namespace = "http://www.w3.org/2000/svg";
  const icon = document.createElementNS(namespace, "svg");
  icon.setAttribute("class", "device-icon");
  icon.setAttribute("viewBox", "0 0 24 24");
  icon.setAttribute("aria-hidden", "true");
  icon.setAttribute("focusable", "false");
  for (const pathData of DEVICE_ICON_PATHS[kind] || DEVICE_ICON_PATHS.browser) {
    const path = document.createElementNS(namespace, "path");
    path.setAttribute("d", pathData);
    icon.append(path);
  }
  return icon;
}

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

function selectedAttachment() {
  return photoInput.files[0] || fileInput.files[0] || null;
}

function clearAttachment() {
  photoInput.value = "";
  fileInput.value = "";
}

function setAttachmentMenuOpen(open) {
  attachmentMenu.hidden = !open;
  attachButton.setAttribute("aria-expanded", String(open));
  if (open) choosePhotosButton.focus();
}

function showAccess(message = "", clearCredential = true, passwordRequired = false) {
  authController.abort();
  authController = new AbortController();
  authGeneration += 1;
  currentDevice = null;
  setAttachmentMenuOpen(false);
  if (clearCredential) {
    deviceToken = "";
    // localStorage is shared by every same-origin tab. A stale tab must not
    // delete a newer token written by another tab; a successful join replaces it.
    textInput.value = "";
    clearAttachment();
    activityStatus = "";
    sendError = "";
    storageError = "";
    resizeComposer();
    updateComposer();
  }
  accessElement.hidden = false;
  sidebar.hidden = true;
  passwordField.hidden = !passwordRequired;
  conversationElement.hidden = true;
  composerShell.hidden = true;
  deviceButton.hidden = true;
  connectionElement.textContent = passwordRequired ? "Password required" : "Connect";
  accessError.textContent = message;
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
  accessElement.hidden = true;
  sidebar.hidden = false;
  conversationElement.hidden = false;
  composerShell.hidden = false;
  deviceButton.hidden = false;
  currentDeviceElement.textContent = `Current device: ${device.name}`;
  connectionElement.textContent = "Local";
  accessError.textContent = "";
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
  const selected = selectedAttachment();
  fileChip.hidden = !selected;
  fileName.textContent = selected?.name || "";
  textInput.disabled = Boolean(selected);
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
  article.classList.toggle("is-current-device", message.is_current_device === true);
  article.dataset.messageId = message.id;

  const body = document.createElement("div");
  body.className = "message-body";
  const head = document.createElement("div");
  head.className = "message-head";
  const sourceIcon = document.createElement("span");
  sourceIcon.className = "message-source-icon";
  sourceIcon.append(createDeviceIcon(message.sender_kind));
  sourceIcon.setAttribute("aria-hidden", "true");
  const sender = document.createElement("span");
  sender.className = "sender";
  sender.textContent = message.sender_name;
  const time = document.createElement("time");
  time.className = "time";
  time.dateTime = message.created_at;
  time.textContent = new Date(message.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  head.append(sourceIcon, sender, time);

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

  article.append(body, head);
  messagesElement.append(article);
}

async function readError(response) {
  return (await responseError(response)).message;
}

async function responseError(response) {
  try {
    const body = await response.json();
    const error = new Error(body.error?.message || `Request failed (${response.status})`);
    error.code = body.error?.code || "";
    return error;
  } catch {
    return new Error(`Request failed (${response.status})`);
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
      showAccess("This device is no longer connected.");
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
      showAccess("This device is no longer connected.");
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
  if (response.status === 401) showAccess("This device is no longer connected.");
  if (!response.ok) throw new Error(await readError(response));
  return true;
}

async function sendFile(file) {
  const generation = authGeneration;
  const form = new FormData();
  form.append("file", file, file.name);
  const response = await authenticatedFetch("/api/v1/messages/file", { method: "POST", body: form, signal: authController.signal });
  if (generation !== authGeneration) return false;
  if (response.status === 401) showAccess("This device is no longer connected.");
  if (!response.ok) throw new Error(await readError(response));
  return true;
}

async function loadSession() {
  if (sessionLoading) return;
  if (!deviceToken) {
    await beginAccess();
    return;
  }
  const generation = authGeneration;
  const token = deviceToken;
  sessionLoading = true;
  try {
    const response = await authenticatedFetch("/api/v1/session", { cache: "no-store", signal: authController.signal });
    if (generation !== authGeneration || token !== deviceToken) return;
    if (response.status === 401) {
      await beginAccess();
      return;
    }
    if (!response.ok) throw new Error(await readError(response));
    const payload = await response.json();
    showApp(payload.device);
    await loadMessages();
  } catch (error) {
    if (generation !== authGeneration || token !== deviceToken || error.name === "AbortError") return;
    showAccess(error.message, false);
    connectionElement.textContent = "Offline";
  } finally {
    sessionLoading = false;
  }
}

async function beginAccess() {
  try {
    const response = await fetch("/api/v1/access", { cache: "no-store" });
    if (!response.ok) throw new Error(await readError(response));
    const payload = await response.json();
    if (payload.password_required) {
      showAccess("", true, true);
    } else {
      try {
        await joinAccess("");
      } catch (error) {
        // The setting can change after the public status read. Never leave a
        // credential-less browser at a dead end with its password field hidden.
        showJoinFailure(error);
      }
    }
  } catch (error) {
    showAccess(error.message, false);
    connectionElement.textContent = "Offline";
  }
}

function showJoinFailure(error) {
  const passwordRequired = error.code === "invalid_password";
  const keepPasswordField = passwordRequired || !passwordField.hidden;
  showAccess(error.message, true, keepPasswordField);
  if (!passwordRequired) connectionElement.textContent = "Offline";
}

async function joinAccess(password) {
  if (!storageWritable()) throw new Error("Browser storage is unavailable. Enable site storage before connecting this device.");
  const response = await fetch("/api/v1/access/join", {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Ferry-Device-Kind": localDeviceKind },
    body: JSON.stringify({ device_name: deviceNameInput.value, password }),
  });
  if (!response.ok) throw await responseError(response);
  const payload = await response.json();
  accessPasswordInput.value = "";
  resetTimeline();
  deviceToken = payload.token;
  storageError = storeToken(payload.token) ? "" : "Browser storage is unavailable. Keep this tab open until you reconnect.";
  showApp(payload.device);
  await loadMessages();
}

function pollServer() {
  if (currentDevice) {
    loadMessages();
  } else if (deviceToken) {
    loadSession();
  }
}

accessForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const submit = accessForm.querySelector("button[type=submit]");
  submit.disabled = true;
  accessError.textContent = "";
  try {
    await joinAccess(accessPasswordInput.value);
  } catch (error) {
    showJoinFailure(error);
  } finally {
    submit.disabled = false;
  }
});

async function loadDevices() {
  const generation = authGeneration;
  const response = await authenticatedFetch("/api/v1/devices", { cache: "no-store", signal: authController.signal });
  if (generation !== authGeneration) return;
  if (response.status === 401) {
    showAccess("This device is no longer connected.");
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
    meta.textContent = device.id === currentDevice.id ? "This device" : `Connected ${new Date(device.created_at).toLocaleDateString()}`;
    copy.append(name, meta);
    const identity = document.createElement("div");
    identity.className = "device-identity";
    const icon = document.createElement("div");
    icon.className = "device-list-icon";
    icon.append(createDeviceIcon(device.kind));
    identity.append(icon, copy);
    row.append(identity);
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
            showAccess("This device is no longer connected.");
            return;
          }
          if (!result.ok) throw new Error(await readError(result));
          await loadDevices();
        } catch (error) {
          if (generation !== authGeneration || error.name === "AbortError") return;
          accessSettingsStatus.textContent = error.message;
          revoke.disabled = false;
        }
      });
      row.append(revoke);
    }
    deviceList.append(row);
  }
}

deviceButton.addEventListener("click", async () => {
  accessSettingsStatus.textContent = "";
  settingsPasswordInput.value = "";
  devicesDialog.showModal();
  try {
    const [, response] = await Promise.all([
      loadDevices(),
      authenticatedFetch("/api/v1/settings/access", { cache: "no-store", signal: authController.signal }),
    ]);
    if (response.status === 401) {
      showAccess("This device is no longer connected.");
      return;
    }
    if (!response.ok) throw new Error(await readError(response));
    const setting = await response.json();
    accessSettingsStatus.textContent = setting.password_required ? "Password is enabled." : "No password is required.";
  } catch (error) {
    if (error.name === "AbortError") return;
    accessSettingsStatus.textContent = error.message;
  }
});

closeDevicesButton.addEventListener("click", () => devicesDialog.close());

saveAccessButton.addEventListener("click", async () => {
  saveAccessButton.disabled = true;
  accessSettingsStatus.textContent = "Saving…";
  const generation = authGeneration;
  try {
    const response = await authenticatedFetch("/api/v1/settings/access", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password: settingsPasswordInput.value }),
      signal: authController.signal,
    });
    if (generation !== authGeneration) return;
    if (response.status === 401) {
      showAccess("This device is no longer connected.");
      return;
    }
    if (!response.ok) throw new Error(await readError(response));
    const setting = await response.json();
    settingsPasswordInput.value = "";
    accessSettingsStatus.textContent = setting.password_required ? "Password enabled for new devices." : "Password disabled; new devices join directly.";
  } catch (error) {
    if (generation !== authGeneration || error.name === "AbortError") return;
    accessSettingsStatus.textContent = error.message;
  } finally {
    saveAccessButton.disabled = false;
  }
});

composer.addEventListener("submit", async (event) => {
  event.preventDefault();
  const selectedFile = selectedAttachment();
  const text = textInput.value;
  const generation = authGeneration;
  sendButton.disabled = true;
  sendError = "";
  activityStatus = selectedFile ? "Sending file…" : "Sending…";
  renderStatus();
  try {
    if (selectedFile) {
      if (!await sendFile(selectedFile)) return;
      clearAttachment();
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

attachButton.addEventListener("click", () => setAttachmentMenuOpen(attachmentMenu.hidden));
choosePhotosButton.addEventListener("click", () => {
  setAttachmentMenuOpen(false);
  attachButton.focus();
  photoInput.click();
});
chooseFilesButton.addEventListener("click", () => {
  setAttachmentMenuOpen(false);
  attachButton.focus();
  fileInput.click();
});
photoInput.addEventListener("change", () => {
  if (photoInput.files.length > 0) fileInput.value = "";
  updateComposer();
});
fileInput.addEventListener("change", () => {
  if (fileInput.files.length > 0) photoInput.value = "";
  updateComposer();
});
document.addEventListener("click", (event) => {
  if (!attachmentMenu.hidden && !attachmentPicker.contains(event.target)) setAttachmentMenuOpen(false);
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && !attachmentMenu.hidden) {
    setAttachmentMenuOpen(false);
    attachButton.focus();
  }
});
removeFileButton.addEventListener("click", () => {
  clearAttachment();
  updateComposer();
  textInput.focus();
});
window.addEventListener("resize", resizeComposer);

updateComposer();
resizeComposer();
loadSession();
window.setInterval(pollServer, 1500);
