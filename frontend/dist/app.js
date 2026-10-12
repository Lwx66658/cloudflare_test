const DEFAULT_API_URL = window.location.origin;
const API_URL_KEY = "cloudflare-test-api-url";

const elements = {
  apiUrl: document.querySelector("#api-url"), saveApiUrl: document.querySelector("#save-api-url"),
  checkHealth: document.querySelector("#check-health"), healthDot: document.querySelector("#health-dot"),
  healthText: document.querySelector("#health-text"), notice: document.querySelector("#notice"),
  messageForm: document.querySelector("#message-form"), messageContent: document.querySelector("#message-content"),
  refreshMessages: document.querySelector("#refresh-messages"), latestMessage: document.querySelector("#latest-message"),
  messageList: document.querySelector("#message-list"), messageCount: document.querySelector("#message-count"),
  imageForm: document.querySelector("#image-form"), imageFile: document.querySelector("#image-file"),
  selectedFile: document.querySelector("#selected-file"), refreshImages: document.querySelector("#refresh-images"),
  imageList: document.querySelector("#image-list"), imageCount: document.querySelector("#image-count"),
};

function apiBase() { return elements.apiUrl.value.trim().replace(/\/+$/, ""); }
function endpoint(path) {
  const base = apiBase();
  if (!base) throw new Error("请先填写后端 Worker 地址");
  return `${base}${path}`;
}
function showNotice(message, type = "info") {
  elements.notice.textContent = message;
  elements.notice.className = `notice${type === "info" ? "" : ` notice--${type}`}`;
  elements.notice.hidden = false;
}
function clearNotice() { elements.notice.hidden = true; }
function setBusy(button, busy) { button.disabled = busy; button.setAttribute("aria-busy", String(busy)); }

async function request(path, options = {}) {
  const response = await fetch(endpoint(path), options);
  const contentType = response.headers.get("content-type") || "";
  const body = contentType.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) {
    const detail = typeof body === "object" && body?.error ? body.error : String(body || response.statusText);
    throw new Error(`${response.status} · ${detail}`);
  }
  return body;
}

function formatTime(value) {
  if (!value) return "";
  const normalized = value.includes("T") ? value : `${value.replace(" ", "T")}Z`;
  const date = new Date(normalized);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}
function formatBytes(bytes) {
  if (!Number.isFinite(bytes)) return "";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MiB`;
}
function emptyItem(text) {
  const item = document.createElement("li"); item.className = "empty"; item.textContent = text; return item;
}
function messageItem(message) {
  const item = document.createElement("li"); item.className = "data-item";
  const title = document.createElement("p"); title.className = "data-item__title"; title.textContent = message.content;
  const meta = document.createElement("p"); meta.className = "data-item__meta"; meta.textContent = `#${message.id} · ${formatTime(message.create_time)}`;
  item.append(title, meta); return item;
}
function imageItem(image) {
  const item = document.createElement("li"); item.className = "data-item";
  const row = document.createElement("div"); row.className = "data-item__top";
  const title = document.createElement("p"); title.className = "data-item__title"; title.textContent = image.original_name;
  const download = document.createElement("a"); download.className = "download"; download.textContent = "下载";
  download.href = endpoint(image.download_url || `/images/${image.id}/download`); download.target = "_blank"; download.rel = "noopener";
  const meta = document.createElement("p"); meta.className = "data-item__meta"; meta.textContent = `${formatBytes(image.size)} · ${formatTime(image.created_at)}`;
  row.append(title, download); item.append(row, meta); return item;
}

async function checkHealth() {
  clearNotice(); setBusy(elements.checkHealth, true); elements.healthDot.className = "health__dot"; elements.healthText.textContent = "检查中…";
  try {
    await request("/"); elements.healthDot.className = "health__dot health__dot--ok"; elements.healthText.textContent = "后端连接正常";
  } catch (error) {
    elements.healthDot.className = "health__dot health__dot--error"; elements.healthText.textContent = "连接失败";
    showNotice(`无法连接后端：${error.message}`, "error");
  } finally { setBusy(elements.checkHealth, false); }
}
async function loadMessages() {
  setBusy(elements.refreshMessages, true);
  try {
    const [messages, latest] = await Promise.all([
      request("/messages"),
      request("/messages/latest").catch((error) => error.message.startsWith("404") ? null : Promise.reject(error)),
    ]);
    elements.messageList.replaceChildren(...(messages.length ? messages.map(messageItem) : [emptyItem("暂无消息")]));
    elements.messageCount.textContent = String(messages.length);
    elements.latestMessage.textContent = latest ? `${latest.content} · ${formatTime(latest.create_time)}` : "暂无消息";
  } catch (error) { showNotice(`读取消息失败：${error.message}`, "error"); }
  finally { setBusy(elements.refreshMessages, false); }
}
async function createMessage(event) {
  event.preventDefault(); const button = elements.messageForm.querySelector("button[type=submit]"); setBusy(button, true);
  try {
    await request("/messages", { method: "POST", headers: { "Content-Type": "text/plain;charset=UTF-8" }, body: elements.messageContent.value.trim() });
    elements.messageForm.reset(); showNotice("消息创建成功。", "success"); await loadMessages();
  } catch (error) { showNotice(`创建消息失败：${error.message}`, "error"); }
  finally { setBusy(button, false); }
}
async function loadImages() {
  setBusy(elements.refreshImages, true);
  try {
    const images = await request("/images");
    elements.imageList.replaceChildren(...(images.length ? images.map(imageItem) : [emptyItem("暂无图片")]));
    elements.imageCount.textContent = String(images.length);
  } catch (error) { showNotice(`读取图片列表失败：${error.message}`, "error"); }
  finally { setBusy(elements.refreshImages, false); }
}
async function uploadImage(event) {
  event.preventDefault(); const file = elements.imageFile.files[0]; if (!file) return;
  const button = elements.imageForm.querySelector("button[type=submit]"); const formData = new FormData(); formData.append("image", file); setBusy(button, true);
  try {
    await request("/images", { method: "POST", body: formData }); elements.imageForm.reset();
    elements.selectedFile.textContent = "最大 10 MiB"; showNotice("图片上传成功。", "success"); await loadImages();
  } catch (error) { showNotice(`上传图片失败：${error.message}`, "error"); }
  finally { setBusy(button, false); }
}

elements.apiUrl.value = localStorage.getItem(API_URL_KEY) || DEFAULT_API_URL;
elements.saveApiUrl.addEventListener("click", () => { localStorage.setItem(API_URL_KEY, apiBase()); showNotice("后端地址已保存。", "success"); checkHealth(); });
elements.checkHealth.addEventListener("click", checkHealth);
elements.refreshMessages.addEventListener("click", loadMessages);
elements.messageForm.addEventListener("submit", createMessage);
elements.refreshImages.addEventListener("click", loadImages);
elements.imageForm.addEventListener("submit", uploadImage);
elements.imageFile.addEventListener("change", () => {
  const file = elements.imageFile.files[0]; elements.selectedFile.textContent = file ? `${file.name} · ${formatBytes(file.size)}` : "最大 10 MiB";
});
checkHealth(); loadMessages(); loadImages();
