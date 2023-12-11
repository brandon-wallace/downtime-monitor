// script.js

let wsStatus = document.querySelector(".websocket-status");
let currentTime = document.querySelector(".time");
let statusCode = document.querySelector(".status-code");

// Modal window
const dialog = document.getElementById("dialogModal");
const openModalButton = document.getElementById("openModal");
const cancelButton = document.getElementById("cancel");
const submitButton = document.getElementById("submit");

// Form data - Name, URL, Interval
let sitename = document.getElementById("name");
let url = document.getElementById("url"); 
let interval = document.getElementById("interval");
let socket = new WebSocket("ws://localhost:8080/echo");

// openModalButton button opens the modal window.
openModalButton.addEventListener("click", () => {
    dialog.showModal();
});

// cancelButton button closes the modal window.
cancelButton.addEventListener("click", () => {
    dialog.close("Nothing selected");
});

// Websocket
socket.onopen = (e) => {
    wsStatus.textContent = "connected";
}

socket.onmessage = (e) => {
    let ws = JSON.parse(e.data);
    updateTable(ws)
};

socket.onclose = (e) => {
    if (e.wasClean) {
        wsStatus.innerHTML = "disconnected";
        console.log(e.close, e.reason);
    } else {
        wsStatus.innerHTML = "disconnected";
        console.log("ws connection died");
    }
};

socket.onerror = (err) => {
    wsStatus.textContent = "websocket error";
    console.log(err);
    socket.send(`Error ${err}`)
};  

function send() {
    socket.send(JSON.stringify({'name': sitename.value, 'url': url.value, 'interval': parseInt(interval.value)}));
}

submitButton.addEventListener('click', () => {
    console.log('clicked submit')
    send()
})

function randInt(min, max) {
    return Math.floor(Math.random() * (max - min) + min);
}

function updateTable(ws) {
    const tableRows = [...document.querySelectorAll(".row")];
    for (let i = 0; i < tableRows.length; i++) {
        console.log("WS", ws.id, parseInt(tableRows[i].cells[0].textContent, 10))
        if (parseInt(tableRows[i].cells[0].textContent, 10) == ws.id) {
            tableRows[i].cells[3].textContent = ws.status_code
        }
    }
}
