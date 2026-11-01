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
const filePreview = document.querySelector("#file-preview");
const fileName = document.querySelector("#file-name");
const removeFileButton = document.querySelector("#remove-file");
const statusElement = document.querySelector("#status");
const connectionElement = document.querySelector("#connection");
const accessElement = document.querySelector("#access");
const accessForm = document.querySelector("#access-form");
const accessError = document.querySelector("#access-error");
const httpNotice = document.querySelector("#http-notice");
const passwordField = document.querySelector("#password-field");
const accessPasswordInput = document.querySelector("#access-password");
const deviceNameInput = document.querySelector("#device-name");
const timelineButton = document.querySelector("#timeline-button");
const deviceButton = document.querySelector("#device-button");
const devicesPage = document.querySelector("#devices-page");
const currentDeviceElement = document.querySelector("#current-device");
const deviceList = document.querySelector("#device-list");
const settingsPasswordInput = document.querySelector("#settings-password");
const saveAccessButton = document.querySelector("#save-access");
const accessSettingsStatus = document.querySelector("#access-settings-status");
const welcomeGuide = document.querySelector("#welcome-guide");
const welcomeConnectButton = document.querySelector("#welcome-connect");
const serverAddressInput = document.querySelector("#server-address");
const copyAddressButton = document.querySelector("#copy-address");
const connectStatus = document.querySelector("#connect-status");
const connectHint = document.querySelector("#connect-hint");
const connectQR = document.querySelector("#connect-qr");

const messageImages = new Map();
const pendingImages = new WeakMap();
// Loading every image the moment it renders would fetch a whole backlog at
// once; the margin still starts the fetch before the image is on screen.
const imageObserver = new IntersectionObserver((entries) => {
  for (const entry of entries) {
    if (!entry.isIntersecting) continue;
    imageObserver.unobserve(entry.target);
    loadMessageImage(entry.target);
  }
}, { rootMargin: "400px" });
const IMAGE_LOAD_TIMEOUT_MS = 60000;

let cursor = 0;
let retractionCursor = 0;
const retracted = new Set();
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
let pastedAttachment = null;
let previewAttachment = null;
let previewURL = "";
let previewFailed = false;
let activeView = "timeline";
const rendered = new Set();

const DEVICE_ICON_PATHS = Object.freeze({
  iphone: ["M6 5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2V5", "M11 4h2", "M12 17v.01"],
  ipad: ["M5 4a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v16a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4", "M11 17a1 1 0 1 0 2 0 1 1 0 0 0-2 0"],
  mac: ["M3 5a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V5", "M7 20h10", "M9 16v4", "M15 16v4"],
  android: ["M4 10v6", "M20 10v6", "M7 9h10v8a1 1 0 0 1-1 1H8a1 1 0 0 1-1-1V9a5 5 0 0 1 10 0", "M8 3l1 2", "M16 3l-1 2", "M9 18v3", "M15 18v3"],
  windows: ["M17.8 20l-12-1.5A2 2 0 0 1 4 16.6V7.4a2 2 0 0 1 1.8-1.9l12-1.5A2 2 0 0 1 20 5.9V18a2 2 0 0 1-2.2 1.9V20", "M12 5v14", "M4 12h16"],
  browser: ["M4 8h16", "M4 6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6", "M8 4v4"],
});

// Tabler icons, like the device icons above.
const ACTION_ICON_PATHS = Object.freeze({
  copy: ["M7 9.667A2.667 2.667 0 0 1 9.667 7h8.666A2.667 2.667 0 0 1 21 9.667v8.666A2.667 2.667 0 0 1 18.333 21H9.667A2.667 2.667 0 0 1 7 18.333z", "M4.012 16.737A2.005 2.005 0 0 1 3 15V5c0-1.1.9-2 2-2h10c.75 0 1.158.385 1.5 1"],
  copied: ["M5 12l5 5L20 7"],
  recall: ["M4 7h16", "M10 11v6", "M14 11v6", "M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2l1-12", "M9 7V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v3"],
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
  return createIcon(DEVICE_ICON_PATHS[kind] || DEVICE_ICON_PATHS.browser, "device-icon");
}

function createIcon(paths, className) {
  const namespace = "http://www.w3.org/2000/svg";
  const icon = document.createElementNS(namespace, "svg");
  icon.setAttribute("class", className);
  icon.setAttribute("viewBox", "0 0 24 24");
  icon.setAttribute("aria-hidden", "true");
  icon.setAttribute("focusable", "false");
  for (const pathData of paths) {
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
  retractionCursor = 0;
  retracted.clear();
  rendered.clear();
  for (const objectURL of messageImages.values()) URL.revokeObjectURL(objectURL);
  messageImages.clear();
  imageObserver.disconnect();
  messagesElement.replaceChildren();
  welcome.hidden = false;
  // The guide belongs to a timeline known to be empty, not one still loading.
  welcomeGuide.hidden = true;
}

function selectedAttachment() {
  return pastedAttachment || photoInput.files[0] || fileInput.files[0] || null;
}

function clearAttachment() {
  pastedAttachment = null;
  photoInput.value = "";
  fileInput.value = "";
}

function pastedImage(event) {
  const files = Array.from(event.clipboardData?.files || []);
  const directImage = files.find((file) => file.type.startsWith("image/"));
  if (directImage) return directImage;
  for (const item of Array.from(event.clipboardData?.items || [])) {
    if (item.kind === "file" && item.type.startsWith("image/")) return item.getAsFile();
  }
  return null;
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
    activeView = "timeline";
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
  // Behind a deployer's TLS proxy the page itself is HTTPS, and the warning
  // would be false.
  httpNotice.hidden = location.protocol === "https:";
  conversationElement.hidden = true;
  devicesPage.hidden = true;
  composerShell.hidden = true;
  deviceButton.hidden = true;
  connectionElement.textContent = passwordRequired ? "Password required" : "Connect";
  accessError.textContent = message;
  connectionError = "";
}

function selectView(view) {
  activeView = view;
  const connected = Boolean(currentDevice);
  const timelineActive = connected && view === "timeline";
  const devicesActive = connected && view === "devices";
  conversationElement.hidden = !timelineActive;
  composerShell.hidden = !timelineActive;
  devicesPage.hidden = !devicesActive;
  timelineButton.classList.toggle("is-active", timelineActive);
  deviceButton.classList.toggle("is-active", devicesActive);
  if (timelineActive) timelineButton.setAttribute("aria-current", "page");
  else timelineButton.removeAttribute("aria-current");
  if (devicesActive) deviceButton.setAttribute("aria-current", "page");
  else deviceButton.removeAttribute("aria-current");
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
  deviceButton.hidden = false;
  currentDeviceElement.textContent = `Current device: ${device.name}`;
  connectionElement.textContent = "Local";
  accessError.textContent = "";
  selectView(activeView);
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
  if (selected !== previewAttachment) {
    if (previewURL) URL.revokeObjectURL(previewURL);
    previewAttachment = selected;
    previewURL = selected?.type.startsWith("image/") ? URL.createObjectURL(selected) : "";
    previewFailed = false;
  }
  const isImage = Boolean(previewURL) && !previewFailed;
  fileChip.hidden = !selected;
  composer.classList.toggle("has-attachment", Boolean(selected));
  fileChip.classList.toggle("is-image", isImage);
  fileChip.setAttribute("aria-label", selected ? `Selected attachment: ${selected.name}` : "");
  filePreview.hidden = !isImage;
  if (isImage) filePreview.src = previewURL;
  else filePreview.removeAttribute("src");
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
  if (rendered.has(message.id) || retracted.has(message.id)) return;
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

  const actions = document.createElement("div");
  actions.className = "message-actions";
  if (message.kind === "text") {
    const text = document.createElement("p");
    text.className = "message-text";
    text.textContent = message.text;
    body.append(text);
    actions.append(copyControl(text));
  } else if (message.kind === "file") {
    if (isImageFile(message.file)) {
      // A class rather than :has(), which this stylesheet does not rely on.
      body.classList.add("has-image");
      body.append(imageAttachment(message));
    } else {
      body.append(fileCard(message));
    }
  } else {
    return;
  }

  actions.append(retractControl(message));
  article.append(head, body, actions);
  messagesElement.append(article);
}

function removeMessage(id) {
  retracted.add(id);
  const article = messagesElement.querySelector(`[data-message-id="${id}"]`);
  for (const figure of article?.querySelectorAll("figure") || []) imageObserver.unobserve(figure);
  article?.remove();
  const objectURL = messageImages.get(id);
  if (objectURL) {
    document.querySelector(`.viewer[data-message-id="${id}"]`)?.dispatchEvent(new Event("click"));
    URL.revokeObjectURL(objectURL);
    messageImages.delete(id);
  }
  rendered.delete(id);
  welcome.hidden = rendered.size !== 0;
}

function retractControl(message) {
  const button = actionButton("Recall", ACTION_ICON_PATHS.recall);
  button.addEventListener("click", async () => {
    if (!window.confirm("Recall this message from all devices? Copies and downloads already saved will remain.")) return;
    const generation = authGeneration;
    button.disabled = true;
    setActionLabel(button, "Recalling…");
    try {
      const response = await authenticatedFetch(`/api/v1/messages/${message.id}`, { method: "DELETE", signal: authController.signal });
      if (generation !== authGeneration) return;
      if (response.status === 401) { showAccess("This device is no longer connected."); return; }
      if (!response.ok) throw new Error(await readError(response));
      removeMessage(message.id);
      sendError = "";
      renderStatus();
    } catch (error) {
      if (generation !== authGeneration || error.name === "AbortError") return;
      sendError = `Could not recall message: ${error.message}`;
      renderStatus();
    } finally {
      button.disabled = false;
      setActionLabel(button, "Recall");
    }
  });
  return button;
}

function isImageFile(file) {
  return typeof file.media_type === "string" && file.media_type.startsWith("image/");
}

function fileCard(message) {
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
  return link;
}

// The bytes need the device token, so an <img src> pointing at the download URL
// would come back 401. Every image is fetched, turned into an object URL and
// revoked when the timeline resets.
function imageAttachment(message) {
  const figure = document.createElement("figure");
  figure.className = "message-image is-loading";
  figure.setAttribute("aria-busy", "true");
  const image = document.createElement("img");
  image.alt = message.file.name;
  image.decoding = "async";
  figure.append(image);
  pendingImages.set(figure, message);
  imageObserver.observe(figure);
  return figure;
}

// The observer fires once per figure, so an image whose request died without
// an answer has to be queued again or it stays a loading placeholder for good.
async function loadMessageImage(figure) {
  const message = pendingImages.get(figure);
  if (!message || retracted.has(message.id)) return;
  const generation = authGeneration;
  const authSignal = authController.signal;
  const request = new AbortController();
  const abortRequest = () => request.abort();
  authSignal.addEventListener("abort", abortRequest, { once: true });
  // A request to a laptop that went to sleep or a dropped Wi-Fi link can hang
  // with no error at all; give up and offer the file card instead.
  let timedOut = false;
  const timer = setTimeout(() => {
    timedOut = true;
    request.abort();
  }, IMAGE_LOAD_TIMEOUT_MS);
  try {
    const response = await authenticatedFetch(message.file.download_url, { cache: "no-store", signal: request.signal });
    if (generation !== authGeneration) return retryMessageImage(figure);
    if (!response.ok) throw new Error(await readError(response));
    const blob = await response.blob();
    if (generation !== authGeneration) return retryMessageImage(figure);
    if (retracted.has(message.id) || !figure.isConnected) return;
    const objectURL = URL.createObjectURL(blob);
    messageImages.set(message.id, objectURL);
    const image = figure.querySelector("img");
    image.addEventListener("load", () => {
      figure.classList.remove("is-loading");
      figure.removeAttribute("aria-busy");
    }, { once: true });
    image.addEventListener("error", () => fallBackToFileCard(figure, message), { once: true });
    image.addEventListener("click", () => openViewer(objectURL, message.file.name, message.id));
    image.src = objectURL;
  } catch (error) {
    // A reconnect aborts every request in flight; the image loads again once
    // it is on screen under the new connection.
    if (!timedOut && (generation !== authGeneration || error.name === "AbortError")) return retryMessageImage(figure);
    fallBackToFileCard(figure, message);
  } finally {
    clearTimeout(timer);
    authSignal.removeEventListener("abort", abortRequest);
  }
}

function retryMessageImage(figure) {
  if (figure.isConnected) imageObserver.observe(figure);
}

// The bubble tightened its padding for an image; a card needs it back.
function fallBackToFileCard(figure, message) {
  figure.closest(".message-body")?.classList.remove("has-image");
  figure.replaceWith(fileCard(message));
}

function openViewer(source, name, messageID) {
  const overlay = document.createElement("div");
  overlay.className = "viewer";
  overlay.dataset.messageId = messageID;
  overlay.setAttribute("role", "dialog");
  overlay.setAttribute("aria-label", name);
  const image = document.createElement("img");
  image.src = source;
  image.alt = name;
  // Rendering the image inline replaced the file card, which was the only way
  // to save it; the viewer carries that back rather than leaving the reader to
  // find the browser's own context menu.
  const save = document.createElement("a");
  save.className = "viewer-save";
  save.textContent = "Save";
  save.href = source;
  save.download = name;
  save.addEventListener("click", (event) => event.stopPropagation());
  overlay.append(image, save);
  const dismiss = (event) => {
    if (event.type === "keydown" && event.key !== "Escape") return;
    overlay.remove();
    document.removeEventListener("keydown", dismiss);
  };
  overlay.addEventListener("click", dismiss);
  document.addEventListener("keydown", dismiss);
  document.body.append(overlay);
}

function actionButton(label, paths) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "message-action";
  button.append(createIcon(paths, "action-icon"));
  setActionLabel(button, label);
  return button;
}

function setActionLabel(button, label) {
  button.title = label;
  button.setAttribute("aria-label", label);
}

function copyControl(textElement) {
  const button = actionButton("Copy", ACTION_ICON_PATHS.copy);
  const show = (label, paths) => {
    button.replaceChildren(createIcon(paths, "action-icon"));
    setActionLabel(button, label);
  };
  button.addEventListener("click", () => {
    if (copyToClipboard(textElement.textContent)) {
      show("Copied", ACTION_ICON_PATHS.copied);
    } else {
      // Leave the text selected so the platform's copy shortcut still works.
      getSelection().selectAllChildren(textElement);
      show("Text selected; press your copy shortcut", ACTION_ICON_PATHS.copy);
    }
    clearTimeout(button.dataset.timer);
    button.dataset.timer = setTimeout(() => show("Copy", ACTION_ICON_PATHS.copy), 1600);
  });
  return button;
}

// Ferry serves plain HTTP, where navigator.clipboard does not exist at all:
// browsers expose it only to secure contexts. A synchronous execCommand inside
// the click is the one path that works in both contexts, and unlike the async
// API it cannot be deferred until the window is frontmost -- the click already
// proves that it is.
function copyToClipboard(text) {
  const holder = document.createElement("textarea");
  holder.value = text;
  holder.setAttribute("readonly", "");
  holder.style.position = "fixed";
  holder.style.top = "-1000px";
  document.body.append(holder);
  holder.select();
  try {
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    holder.remove();
  }
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

// Saving a fetched blob: URL fails in Android in-app WebViews, which hand the
// blob: URL to the system and find no app for it. The Server instead issues a
// short-lived link that needs no Bearer header, and the browser navigates to
// it, so its own downloader (or the system browser it hands off to) saves it.
async function downloadMessageFile(message) {
  const generation = authGeneration;
  activityStatus = "Downloading…";
  sendError = "";
  renderStatus();
  try {
    const response = await authenticatedFetch(`${message.file.download_url}/link`, { method: "POST", cache: "no-store", signal: authController.signal });
    if (generation !== authGeneration) return;
    if (response.status === 401) {
      showAccess("This device is no longer connected.");
      return;
    }
    if (!response.ok) throw new Error(await readError(response));
    const link = await response.json();
    if (generation !== authGeneration) return;
    const download = document.createElement("a");
    download.href = link.url;
    download.download = message.file.name;
    document.body.append(download);
    download.click();
    download.remove();
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
    const response = await authenticatedFetch(`/api/v1/messages?after=${cursor}&after_retraction=${retractionCursor}&limit=200`, { cache: "no-store", signal: authController.signal });
    if (generation !== authGeneration) return;
    if (response.status === 401) {
      showAccess("This device is no longer connected.");
      return;
    }
    if (!response.ok) throw new Error(await readError(response));
    const payload = await response.json();
    if (generation !== authGeneration) return;
    for (const id of payload.retracted_ids || []) removeMessage(id);
    for (const message of payload.messages) renderMessage(message);
    cursor = payload.next_cursor;
    retractionCursor = payload.next_retraction_cursor || 0;
    welcomeGuide.hidden = false;
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

timelineButton.addEventListener("click", () => selectView("timeline"));

deviceButton.addEventListener("click", openDevices);
welcomeConnectButton.addEventListener("click", openDevices);

async function openDevices() {
  selectView("devices");
  accessSettingsStatus.textContent = "";
  settingsPasswordInput.value = "";
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
}

// The address another device should open is the one this browser is using.
// location.origin carries no path, query or fragment, and the device token
// lives in localStorage, so neither the token nor a password can leak into it.
function renderConnectSection() {
  const address = location.origin;
  serverAddressInput.value = address;
  connectQR.replaceChildren();
  connectQR.hidden = true;
  if (isLocalOnlyHost(location.hostname)) {
    connectHint.textContent = "This page is open at an address only this computer can reach. Open Ferry through the server's LAN IP, such as http://192.168.1.20:42817, then connect other devices from there.";
    return;
  }
  try {
    connectQR.append(qrCodeSVG(address));
    connectQR.hidden = false;
    connectHint.textContent = "";
  } catch {
    connectQR.replaceChildren();
    connectHint.textContent = "The QR code could not be created. Type or copy the address on the other device instead.";
  }
}

function isLocalOnlyHost(hostname) {
  const host = hostname.replace(/^\[|\]$/g, "").toLowerCase();
  return host === "localhost" || host.endsWith(".localhost") || /^127\./.test(host) || /^0\.0\.0\.0$/.test(host) ||
    host === "::1" || host === "::" || host.startsWith("::ffff:7f") || host.startsWith("::ffff:127.");
}

function qrCodeSVG(text) {
  const code = qrcodegen.QrCode.encodeText(text, qrcodegen.QrCode.Ecc.MEDIUM);
  const border = 4;
  const size = code.size + border * 2;
  const namespace = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(namespace, "svg");
  svg.setAttribute("viewBox", `0 0 ${size} ${size}`);
  svg.setAttribute("role", "img");
  svg.setAttribute("aria-label", `QR code for ${text}`);
  svg.setAttribute("shape-rendering", "crispEdges");
  const ground = document.createElementNS(namespace, "rect");
  ground.setAttribute("width", String(size));
  ground.setAttribute("height", String(size));
  ground.setAttribute("fill", "#ffffff");
  let modules = "";
  for (let y = 0; y < code.size; y += 1) {
    for (let x = 0; x < code.size; x += 1) {
      if (code.getModule(x, y)) modules += `M${x + border},${y + border}h1v1h-1z`;
    }
  }
  const path = document.createElementNS(namespace, "path");
  path.setAttribute("d", modules);
  path.setAttribute("fill", "#000000");
  svg.append(ground, path);
  return svg;
}

copyAddressButton.addEventListener("click", () => {
  const copied = copyToClipboard(serverAddressInput.value);
  connectStatus.textContent = copied ? "Address copied." : "Copy failed. Select the address and copy it yourself.";
  connectStatus.classList.toggle("error", !copied);
  if (!copied) {
    serverAddressInput.focus();
    serverAddressInput.select();
  }
});
serverAddressInput.addEventListener("focus", () => serverAddressInput.select());

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

textInput.addEventListener("paste", (event) => {
  const image = pastedImage(event);
  if (!image) return;
  event.preventDefault();
  pastedAttachment = image;
  photoInput.value = "";
  fileInput.value = "";
  setAttachmentMenuOpen(false);
  updateComposer();
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
  if (photoInput.files.length > 0) {
    pastedAttachment = null;
    fileInput.value = "";
  }
  updateComposer();
});
fileInput.addEventListener("change", () => {
  if (fileInput.files.length > 0) {
    pastedAttachment = null;
    photoInput.value = "";
  }
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
filePreview.addEventListener("error", () => {
  if (!previewURL || filePreview.currentSrc !== previewURL) return;
  previewFailed = true;
  updateComposer();
});
window.addEventListener("resize", resizeComposer);

updateComposer();
resizeComposer();
renderConnectSection();
loadSession();
window.setInterval(pollServer, 1500);
