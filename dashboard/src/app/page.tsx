export default function Home() {
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
            line-height: 1.6; color: #e2e8f0;
            background: #0a0a0f;
            min-height: 100vh;
        }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }

        /* Header / Hero */
        .header {
            text-align: center; color: white; margin-bottom: 50px; padding: 60px 20px;
            background: rgba(255,255,255,0.03); border-radius: 20px;
            backdrop-filter: blur(15px);
            border: 1px solid rgba(255,255,255,0.06);
        }
        .header h1 {
            font-size: 3.5rem; margin-bottom: 10px;
            background: linear-gradient(135deg, #60a5fa, #a78bfa);
            -webkit-background-clip: text; -webkit-text-fill-color: transparent;
        }
        .header .subtitle { font-size: 1.3rem; opacity: 0.7; margin-bottom: 40px; }

        /* Portal CTA buttons */
        .portal-ctas { display: flex; gap: 20px; justify-content: center; flex-wrap: wrap; }
        .portal-btn {
            display: flex; flex-direction: column; align-items: center;
            padding: 28px 36px; border-radius: 16px; text-decoration: none;
            transition: all 0.3s ease; min-width: 240px; border: 1px solid rgba(255,255,255,0.08);
        }
        .portal-btn:hover { transform: translateY(-3px); }
        .portal-btn .portal-icon { font-size: 2.4rem; margin-bottom: 10px; }
        .portal-btn .portal-title { font-size: 1.2rem; font-weight: 700; margin-bottom: 4px; }
        .portal-btn .portal-tag {
            font-size: 0.7rem; font-weight: 600; letter-spacing: 0.08em;
            padding: 2px 10px; border-radius: 999px; margin-bottom: 8px; text-transform: uppercase;
        }
        .portal-btn .portal-desc { font-size: 0.85rem; opacity: 0.8; }

        /* SSP */
        .portal-ssp {
            background: linear-gradient(135deg, rgba(5,150,105,0.15), rgba(13,148,136,0.08));
            border-color: rgba(16,185,129,0.2); color: #d1fae5;
        }
        .portal-ssp:hover { box-shadow: 0 12px 40px rgba(16,185,129,0.2); border-color: rgba(16,185,129,0.4); }
        .portal-ssp .portal-tag { background: rgba(16,185,129,0.2); color: #6ee7b7; }

        /* DSP */
        .portal-dsp {
            background: linear-gradient(135deg, rgba(124,58,237,0.15), rgba(99,102,241,0.08));
            border-color: rgba(139,92,246,0.2); color: #ede9fe;
        }
        .portal-dsp:hover { box-shadow: 0 12px 40px rgba(139,92,246,0.2); border-color: rgba(139,92,246,0.4); }
        .portal-dsp .portal-tag { background: rgba(139,92,246,0.2); color: #c4b5fd; }

        /* Features grid */
        .features {
            display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 24px; margin: 50px 0;
        }
        .feature-card {
            background: rgba(255,255,255,0.03); padding: 28px; border-radius: 15px;
            text-align: center; border: 1px solid rgba(255,255,255,0.06);
            transition: all 0.3s ease;
        }
        .feature-card:hover {
            transform: translateY(-3px); border-color: rgba(255,255,255,0.12);
            background: rgba(255,255,255,0.05);
        }
        .feature-icon { font-size: 2.6rem; margin-bottom: 16px; display: block; }
        .feature-card h3 { font-size: 1.3rem; margin-bottom: 10px; color: #f1f5f9; }
        .feature-card p { color: #94a3b8; line-height: 1.6; font-size: 0.95rem; }

        /* Footer */
        .footer { text-align: center; color: #475569; margin-top: 50px; padding: 30px; }
        .admin-link {
            display: inline-block; margin-top: 12px; color: #475569; text-decoration: none;
            font-size: 0.8rem; transition: color 0.2s;
        }
        .admin-link:hover { color: #94a3b8; }

        @media (max-width: 768px) {
            .header h1 { font-size: 2.5rem; }
            .header .subtitle { font-size: 1rem; }
            .portal-ctas { flex-direction: column; align-items: center; }
            .portal-btn { width: 100%; max-width: 340px; }
            .features { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <div class="container">
        <header class="header">
            <h1>IPLoop</h1>
            <p class="subtitle">Next-Generation Proxy Infrastructure Platform</p>

            <div class="portal-ctas">
                <a href="/ssp/login" class="portal-btn portal-ssp">
                    <span class="portal-icon">📡</span>
                    <span class="portal-tag">SSP</span>
                    <span class="portal-title">Publisher Login</span>
                    <span class="portal-desc">Monetize your network</span>
                </a>
                <a href="/dsp/login" class="portal-btn portal-dsp">
                    <span class="portal-icon">🎯</span>
                    <span class="portal-tag">DSP</span>
                    <span class="portal-title">Advertiser Login</span>
                    <span class="portal-desc">Access premium proxies</span>
                </a>
            </div>
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
            <a href="/login" class="admin-link">Admin Access</a>
        </footer>
    </div>

    <script>
        if (localStorage.getItem('token')) {
            var portalType = localStorage.getItem('portalType');
            document.querySelectorAll('.portal-btn').forEach(function(btn) {
                btn.setAttribute('href', '/dashboard');
                btn.querySelector('.portal-desc').textContent = 'Go to Dashboard';
            });
        }
    </script>
</body>
</html>`
    }} />
  )
}
