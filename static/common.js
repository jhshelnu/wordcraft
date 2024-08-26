function toast(msg, alertClass) {
    Toastify({
        text: msg,
        duration: 4_000, // ms
        close: true,
        gravity: "top",
        position: "center",
        stopOnFocus: true,
        className: "alert " + alertClass,
        style: {
            "width": "auto",
            "position": "absolute",
            "z-index": "1"
        }
    }).showToast()
}