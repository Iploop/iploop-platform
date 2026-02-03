# IPLoop Dashboard

A modern, professional customer dashboard for the IPLoop proxy platform built with Next.js 14, Tailwind CSS, and shadcn/ui components.

## Features

### 🔐 Authentication
- Email-based login and registration
- Password visibility toggle
- Demo credentials: `demo@iploop.com` / `demo123`

### 📊 Dashboard Home
- Overview statistics (requests, bandwidth, active sessions, success rate)
- Interactive charts with usage trends
- Regional traffic distribution
- Recent activity feed
- System health monitoring

### 🔑 API Key Management
- Generate new API keys with custom names
- View/hide API keys with masked display
- Copy keys to clipboard
- Revoke keys with confirmation
- Security best practices warnings

### 📈 Usage Statistics
- Detailed analytics with multiple time ranges (7 days, 30 days, 3 months, 1 year)
- Interactive charts for requests and bandwidth
- Hourly usage distribution
- HTTP status code breakdown
- Success rate and error tracking
- Top endpoint analytics

### 🌐 Proxy Endpoints
- Global proxy network overview
- Real-time status monitoring (online, offline, maintenance)
- Performance metrics (uptime, latency, load)
- Search and filter functionality
- Copy-paste configuration examples
- Code examples in multiple languages (cURL, Python, JavaScript)

### 💳 Billing & Credits
- Current balance and monthly spending
- Usage cost tracking with charts
- Monthly budget comparisons
- Transaction history
- Plan comparison and upgrades
- Low balance alerts

### ⚙️ Account Settings
- Profile management (name, email, company, timezone)
- Password change with security requirements
- Two-factor authentication toggle
- Session timeout configuration
- IP address whitelist
- Notification preferences
- API configuration settings
- Account deletion (danger zone)

## Tech Stack

- **Framework**: Next.js 14 with App Router
- **Styling**: Tailwind CSS
- **Components**: shadcn/ui components
- **Charts**: Recharts
- **Icons**: Lucide React
- **TypeScript**: Full type safety
- **Dark Theme**: Professional dark mode design

## Getting Started

### Prerequisites
- Node.js 18+ and npm

### Installation

1. Navigate to the project directory:
   ```bash
   cd /root/clawd-secure/iploop-platform/dashboard
   ```

2. Install dependencies:
   ```bash
   npm install
   ```

3. Start the development server:
   ```bash
   npm run dev
   ```

4. Open [http://localhost:3000](http://localhost:3000) (or the next available port) in your browser.

### Build for Production

```bash
npm run build
npm start
```

## Project Structure

```
src/
├── app/                    # Next.js App Router pages
│   ├── api-keys/          # API key management
│   ├── billing/           # Billing and subscription
│   ├── dashboard/         # Main dashboard
│   ├── endpoints/         # Proxy endpoints
│   ├── settings/          # Account settings
│   ├── usage/            # Usage statistics
│   ├── globals.css       # Global styles
│   ├── layout.tsx        # Root layout
│   └── page.tsx          # Auth/login page
├── components/
│   ├── ui/               # Reusable UI components
│   ├── layout.tsx        # Main layout wrapper
│   └── sidebar.tsx       # Navigation sidebar
└── lib/
    └── utils.ts          # Utility functions
```

## Key Features Implementation

### Responsive Design
- Mobile-first approach with responsive grid layouts
- Collapsible sidebar for mobile navigation
- Touch-friendly interface elements

### Professional Styling
- Clean, modern dark theme
- Consistent spacing and typography
- Subtle animations and transitions
- Professional color scheme with semantic colors

### Data Visualization
- Interactive charts with hover states
- Multiple chart types (area, bar, line, pie)
- Responsive chart containers
- Professional data formatting

### User Experience
- Intuitive navigation with active state indicators
- Loading states and feedback
- Copy-to-clipboard functionality
- Search and filter capabilities
- Contextual help and descriptions

## Mock Data

The dashboard currently uses mock data to demonstrate functionality:
- User account: Demo User (demo@iploop.com)
- Sample usage statistics and trends
- Mock API keys and transactions
- Simulated proxy endpoints worldwide
- Example billing and subscription data

## Security Features

- Password strength requirements
- Two-factor authentication interface
- Session timeout management
- IP address whitelisting
- API key security best practices
- Secure credential handling

## Browser Support

- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)

## Contributing

1. Follow the existing code structure and conventions
2. Use TypeScript for type safety
3. Maintain responsive design principles
4. Test across different screen sizes
5. Keep components modular and reusable

## License

This project is part of the IPLoop platform development.

---

**Demo Credentials**: `demo@iploop.com` / `demo123`

The dashboard is fully functional with comprehensive mock data and demonstrates all the features expected in a production-ready proxy platform interface.