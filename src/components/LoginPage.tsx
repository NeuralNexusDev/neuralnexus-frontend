export default function Component() {
    return (
        <div class="mx-auto max-w-sm space-y-6">
            <div class="space-y-2 text-center">
                <h1 class="text-3xl font-bold">Login</h1>
                <p class="text-gray-500 dark:text-gray-400">
                    Enter your username or email below to login
                </p>
            </div>
            <div>
                <div class="space-y-4">
                    <div class="space-y-2">
                        <label
                            class="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                            for="email"
                        >
                            Username or Email
                        </label>
                        <input
                            class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                            id="email"
                            placeholder="user@example.com"
                            type="email"
                        />
                    </div>
                    <div class="space-y-2">
                        <label
                            class="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
                            for="password"
                        >
                            Password
                        </label>
                        <input
                            class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
                            id="password"
                            type="password"
                        />
                    </div>
                    <button
                        class="border inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-primary text-primary-foreground hover:bg-primary/90 h-10 px-4 py-2 w-full"
                        type="submit"
                    >
                        Login
                    </button>
                </div>
                <div
                    data-orientation="horizontal"
                    role="none"
                    class="shrink-0 bg-gray-100 h-[1px] w-full my-8"
                ></div>
                <div class="flex justify-center">
                    <div class="space-y-4">
                        {/* <button class="w-full">Login with Google</button> */}
                        {/* <button class="w-full">Login with Twich</button> */}
                        <button class="bg-discord-blue inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 bg-background hover:bg-accent hover:text-accent-foreground h-10 px-4 py-2 w-full">
                            <span class="mr-2">Login with Discord</span>
                            <img
                                src="https://assets-global.website-files.com/6257adef93867e50d84d30e2/636e0a6ca814282eca7172c6_icon_clyde_white_RGB.svg" // Adjust the path to your discord logo
                                alt="Discord Logo"
                                class="w-6 h-6 mr-2"
                            />
                        </button>
                        <div class="mt-4 text-center text-sm ">
                            <span class="mr-1">Don't have an account?</span>
                            <a class="underline" href="/register">
                                Sign Up
                            </a>
                        </div>
                        <a class="inline-block w-full text-center text-sm underline" href="#">
                            Forgot your password?
                        </a>
                    </div>
                </div>
            </div>
        </div>
    )
}
