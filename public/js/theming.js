// Handle preload theme preference
const currentTheme = getThemePreference();
if (currentTheme === "light") {
    document.documentElement.classList.remove('dark');
} else {
    document.documentElement.classList.add('dark');
}

// Update theme toggle button visibility based on the current theme
window.addEventListener('load', () => {
    if (currentTheme === "light") {
        document.getElementById("light-mode").hidden = true;
        document.getElementById("dark-mode").hidden = false;
    } else {
        document.getElementById("light-mode").hidden = false;
        document.getElementById("dark-mode").hidden = true;
    }
});

function getThemePreference() {
    const userPreference = localStorage.getItem("theme") || "system";
    switch (userPreference) {
        case "light":
        case "dark":
            return userPreference;
        default:
            return matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
    }
}

function saveThemePreference(userPreference) {
    localStorage.setItem("theme", userPreference);
}

function toggleTheme() {
    document.documentElement.classList.toggle('dark');
    const currentTheme = getThemePreference();
    if (currentTheme === "light") {
        document.getElementById("light-mode").hidden = false;
        document.getElementById("dark-mode").hidden = true;
        saveThemePreference("dark");
    } else {
        document.getElementById("light-mode").hidden = true;
        document.getElementById("dark-mode").hidden = false;
        saveThemePreference("light");
    }
}
