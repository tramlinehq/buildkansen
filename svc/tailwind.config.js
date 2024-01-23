module.exports = {
    plugins: [require("daisyui")],
    content: ["./views/**/*.html"],
    daisyui: {themes: ["dim", "dracula"]},
    theme: {
        fontFamily: {
            'avenir': ['Avenir Next', 'sans-serif'],
        },
        extend: {
            fontSize: {
                '2xl': '1.8rem',
                '11xl': '10rem',
            }
        }
    },
}
