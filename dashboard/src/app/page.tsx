export default function Home() {
  // Serve static HTML home page to avoid TypeScript build issues
  return (
    <div dangerouslySetInnerHTML={{
      __html: `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>IPLoop - Next-Generation Proxy Infrastructure</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            line-height: 1.6; color: #333;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
        }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        .header {
            text-align: center; color: white; margin-bottom: 50px; padding: 60px 20px;
            background: rgba(0, 0, 0, 0.1); border-radius: 20px; backdrop-filter: blur(15px);
            border: 1px solid rgba(255, 255, 255, 0.1);
        }
        .header h1 {
            font-size: 3.5rem; margin-bottom: 15px; text-shadow: 2px 2px 4px rgba(0, 0, 0, 0.3);
        }
        .header .subtitle { font-size: 1.4rem; opacity: 0.9; margin-bottom: 30px; }
        .cta-button {
            display: inline-block; background: linear-gradient(45deg, #FF6B6B, #FF8E8E);
            color: white; padding: 15px 40px; text-decoration: none; border-radius: 50px;
            font-size: 1.1rem; font-weight: 600; transition: all 0.3s ease;
            box-shadow: 0 5px 15px rgba(255, 107, 107, 0.4);
        }
        .cta-button:hover {
            transform: translateY(-2px); box-shadow: 0 8px 25px rgba(255, 107, 107, 0.6);
        }
        .features {
            display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 30px; margin: 50px 0;
        }
        .feature-card {
            background: white; padding: 30px; border-radius: 15px; text-align: center;
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.1); transition: all 0.3s ease;
            border: 1px solid rgba(255, 255, 255, 0.1);
        }
        .feature-card:hover {
            transform: translateY(-5px); box-shadow: 0 20px 40px rgba(0, 0, 0, 0.15);
        }
        .feature-icon { font-size: 3rem; margin-bottom: 20px; display: block; }
        .feature-card h3 { font-size: 1.5rem; margin-bottom: 15px; color: #333; }
        .feature-card p { color: #666; line-height: 1.6; }
        .footer { text-align: center; color: white; opacity: 0.8; margin-top: 50px; padding: 30px; }
        @media (max-width: 768px) {
            .header h1 { font-size: 2.5rem; }
            .header .subtitle { font-size: 1.1rem; }
            .features { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <div class="container">
        <header class="header">
            <h1>IPLoop</h1>
            <p class="subtitle">Next-Generation Proxy Infrastructure Platform</p>
            <a href="/login" class="cta-button">Access Platform Dashboard</a>
        </header>

        <div class="features">
            <div class="feature-card">
                <span class="feature-icon">🌍</span>
                <h3>Global Coverage</h3>
                <p>Worldwide proxy network with geographic targeting capabilities. Access content from any location with city-level precision.</p>
            </div>
            <div class="feature-card">
                <span class="feature-icon">⚡</span>
                <h3>High Performance</h3>
                <p>Lightning-fast proxy speeds with optimized routing and minimal latency. Built for enterprise-scale operations.</p>
            </div>
            <div class="feature-card">
                <span class="feature-icon">🔒</span>
                <h3>Enterprise Security</h3>
                <p>Advanced authentication, session management, and secure protocols. Your data and operations stay protected.</p>
            </div>
            <div class="feature-card">
                <span class="feature-icon">🎯</span>
                <h3>Precision Targeting</h3>
                <p>Country, city, and ASN-level targeting with sticky sessions and rotation controls for maximum flexibility.</p>
            </div>
            <div class="feature-card">
                <span class="feature-icon">📱</span>
                <h3>Mobile SDK</h3>
                <p>Pure Java SDK for Android applications with enterprise features and easy integration.</p>
            </div>
            <div class="feature-card">
                <span class="feature-icon">⚙️</span>
                <h3>Advanced Controls</h3>
                <p>Browser profiles, speed requirements, session lifetime management, and real-time monitoring.</p>
            </div>
        </div>

        <footer class="footer">
            <p>&copy; 2026 IPLoop. Next-Generation Proxy Infrastructure Platform.</p>
        </footer>
    </div>

    <script>
        // Simple redirect if user is already logged in
        if (localStorage.getItem('token')) {
            const loginLink = document.querySelector('.cta-button');
            if (loginLink) {
                loginLink.textContent = 'Go to Dashboard';
                loginLink.href = '/dashboard';
            }
        }
    </script>
</body>
</html>`
    }} />
  )
}
