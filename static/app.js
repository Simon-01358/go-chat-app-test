const joinScreen = document.getElementById("join");
const joinForm = document.getElementById("join-form");
const nameInput = document.getElementById("name-input");
const chat = document.getElementById("chat");
const messagesEl = document.getElementById("messages");
const sendForm = document.getElementById("send-form");
const textInput = document.getElementById("text-input");
const presenceEl = document.getElementById("presence");

let myName = "";

// Remember the name between visits
try {
  nameInput.value = localStorage.getItem("chat-name") || "";
} catch (_) {}

joinForm.addEventListener("submit", (e) => {
  e.preventDefault();
  myName = nameInput.value.trim();
  if (!myName) return;
  try { localStorage.setItem("chat-name", myName); } catch (_) {}
  joinScreen.classList.add("hidden");
  chat.classList.remove("hidden");
  textInput.focus();
  connect();
});

function connect() {
  // The browser opens a long-lived connection; the server pushes new messages into it.
  // EventSource reconnects automatically if the connection drops.
  const source = new EventSource("/events");

  source.addEventListener("message", (e) => addMessage(JSON.parse(e.data)));

  source.addEventListener("presence", (e) => {
    const { online } = JSON.parse(e.data);
    presenceEl.textContent = `${online} online`;
    presenceEl.className = "presence live";
  });

  source.onerror = () => {
    presenceEl.textContent = "reconnecting…";
    presenceEl.className = "presence";
  };
}

function addMessage(m) {
  const wasAtBottom =
    messagesEl.scrollTop + messagesEl.clientHeight >= messagesEl.scrollHeight - 40;

  const div = document.createElement("div");
  div.className = "msg" + (m.name === myName ? " mine" : "");

  const meta = document.createElement("div");
  meta.className = "meta";
  const time = new Date(m.time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  meta.textContent = `${m.name} · ${time}`;

  const body = document.createElement("div");
  body.textContent = m.text; // textContent (not innerHTML) so messages can't inject HTML

  div.append(meta, body);
  messagesEl.appendChild(div);

  if (wasAtBottom || m.name === myName) {
    messagesEl.scrollTop = messagesEl.scrollHeight;
  }
}

sendForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const text = textInput.value.trim();
  if (!text) return;
  textInput.value = "";
  try {
    await fetch("/api/messages", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: myName, text }),
    });
  } catch (_) {
    textInput.value = text; // put it back if sending failed
  }
});
