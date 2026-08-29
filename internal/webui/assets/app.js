const messagesElement = document.querySelector("#messages");
const welcome = document.querySelector("#welcome");
const composer = document.querySelector("#composer");
const textInput = document.querySelector("#text");
const fileInput = document.querySelector("#file");
const sendButton = document.querySelector("#send");
const fileChip = document.querySelector("#file-chip");
const fileName = document.querySelector("#file-name");
const removeFileButton = document.querySelector("#remove-file");
const statusElement = document.querySelector("#status");
const connectionElement = document.querySelector("#connection");

let cursor = 0;
let loading = false;
let activityStatus = "";
let connectionError = "";
let sendError = "";
const rendered = new Set();

function setStatus(message, error = false) {
  statusElement.textContent = message;
  statusElement.classList.toggle("error", error);
}

function renderStatus() {
  const error = sendError || connectionError;
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
    const link = document.createElement("a");
    link.className = "file-card";
    link.href = message.file.download_url;
    link.download = message.file.name;
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

async function loadMessages() {
  if (loading) return;
  loading = true;
  const nearBottom = window.innerHeight + window.scrollY >= document.body.offsetHeight - 140;
  try {
    const response = await fetch(`/api/v1/messages?after=${cursor}&limit=200`, { cache: "no-store" });
    if (!response.ok) throw new Error(await readError(response));
    const payload = await response.json();
    for (const message of payload.messages) renderMessage(message);
    cursor = payload.next_cursor;
    connectionElement.textContent = "Local";
    connectionError = "";
    renderStatus();
    if (nearBottom && payload.messages.length > 0) window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
  } catch (error) {
    connectionElement.textContent = "Offline";
    connectionError = error.message;
    renderStatus();
  } finally {
    loading = false;
  }
}

async function sendText(text) {
  const response = await fetch("/api/v1/messages/text", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ sender_name: "Web", text }),
  });
  if (!response.ok) throw new Error(await readError(response));
}

async function sendFile(file) {
  const form = new FormData();
  form.append("sender_name", "Web");
  form.append("file", file, file.name);
  const response = await fetch("/api/v1/messages/file", { method: "POST", body: form });
  if (!response.ok) throw new Error(await readError(response));
}

composer.addEventListener("submit", async (event) => {
  event.preventDefault();
  const selectedFile = fileInput.files[0];
  const text = textInput.value;
  sendButton.disabled = true;
  sendError = "";
  activityStatus = selectedFile ? "Sending file…" : "Sending…";
  renderStatus();
  try {
    if (selectedFile) {
      await sendFile(selectedFile);
      fileInput.value = "";
    } else {
      await sendText(text);
      textInput.value = "";
      resizeComposer();
    }
    activityStatus = "";
    updateComposer();
    await loadMessages();
    window.scrollTo({ top: document.body.scrollHeight, behavior: "smooth" });
    renderStatus();
  } catch (error) {
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
loadMessages();
window.setInterval(loadMessages, 1500);
