module.exports = {
    plugins: [require("daisyui")],
    content: ["./views/**/*.html"],
    daisyui: {themes: ["dim", "dracula"]},
    theme: {
        // Some useful comment
        fontFamily: {
            'avenir': ['Avenir Next', 'sans-serif'],
        },
        extend: {
            fontSize: {
                '11xl': '10rem',
            }
        }
    },
}
