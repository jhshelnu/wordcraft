/** @type {import('tailwindcss').Config} */
module.exports = {
    content: ["./templates/*.gohtml", "./static/*.js"],
    theme: {
        extend: {},
    },
    plugins: [
        require("daisyui"),
    ],
    daisyui: {
        themes: ["synthwave"]
    }
}