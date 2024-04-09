export default function Component() {
    return (
        <div>
            <h1>Login</h1>

            <form>
                <label>
                    Username:
                    <input type="text" name="username" />
                </label>
                <label>
                    Password:
                    <input type="password" name="password" />
                </label>
                <button type="submit">Login</button>

                <a href="/auth/discord">Login with Discord</a>
                <p>
                    Don't have an account? <a href="/register">Register</a>
                </p>
                <p>
                    Forgot your password? <a href="/forgot-password">Reset</a>
                </p>
            </form>
        </div>
    )
}
