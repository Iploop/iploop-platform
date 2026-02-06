# Ultron Management Platform - Architecture

## Vision
Central management hub for the entire AI-powered organization. Each department has its own section with tools, content, and agent configuration.

## Navigation Structure

```
📊 Dashboard          - Company-wide overview, KPIs, alerts
│
├── 🤖 AI Operations
│   ├── Team Structure   - Interactive org chart (click to configure agents)
│   ├── Agent Config     - Detailed agent settings, prompts, tools
│   └── Build Guide      - How to create new agents/teams
│
├── 🧪 Testing
│   ├── Test Dashboard   - Live results, charts
│   ├── Live Runner      - Real-time test execution
│   └── History          - Historical comparisons
│
├── 💼 Sales
│   ├── CRM              - Lead pipeline, contacts, deals
│   ├── Email Templates  - Outreach sequences, follow-ups
│   ├── Research         - Company/contact research notes
│   └── Reports          - Sales metrics, forecasts
│
├── 📣 Marketing
│   ├── Content Library  - Blog posts, ad copy, landing pages
│   ├── Email Templates  - Marketing emails, newsletters
│   ├── Social Calendar  - Scheduled posts, campaigns
│   ├── Brand Assets     - Logos, guidelines, templates
│   └── Analytics        - Campaign performance
│
├── ⚙️ Development
│   ├── Projects         - All apps/websites with status
│   ├── Repositories     - GitHub repos, branches
│   ├── Deployments      - CI/CD status, environments
│   └── QA Board         - Bugs, tests, quality metrics
│
├── 🤝 Partners
│   ├── Accounts         - Partner list, health scores
│   ├── Support          - Tickets, chat history
│   ├── Onboarding       - New partner setup
│   └── Monitoring       - Usage, alerts
│
├── 💰 Finance
│   ├── Overview         - Revenue, expenses, P&L
│   ├── Invoices         - Generate, track, send
│   ├── Payments         - Incoming, outgoing
│   └── Reports          - Financial statements
│
└── ⚡ System
    ├── Ultron Specs     - Model, hardware, capabilities
    ├── Integrations     - Connected services
    ├── Logs             - Activity, errors
    └── Settings         - Global configuration
```

## Key Features Per Section

### Dashboard (Main)
- Total revenue / MRR widget
- Active leads count
- Support tickets open
- System health status
- Recent activity feed
- Quick actions

### AI Operations
- **Interactive Diagram**: Click any agent box to open config modal
- **Agent Config Modal**:
  - Name, role, description
  - Model selection (Opus/Sonnet)
  - System prompt editor
  - Tools access checkboxes
  - Schedule (cron patterns)
  - Connected integrations
- **Live Status**: Which agents are running, last activity

### Sales CRM
- **Pipeline Board**: Kanban (New → Qualified → Proposal → Negotiation → Won/Lost)
- **Contact Cards**: Name, company, email, notes, activity
- **Email Templates**: 
  - Initial outreach
  - Follow-up sequences
  - Meeting requests
  - Proposals
- **Quick Actions**: Add lead, send email, schedule follow-up

### Marketing Content
- **Content Types**:
  - Blog posts (draft/published)
  - Ad copy (Facebook, Google, LinkedIn)
  - Landing page copy
  - Email campaigns
  - Social posts
  - Slogans/taglines
- **Template Editor**: Rich text, variables, preview
- **Content Calendar**: Visual schedule

### Development Projects
- **Project Cards**:
  - Name, description
  - Status (Planning/Active/Paused/Complete)
  - Tech stack tags
  - Last updated
  - Links (repo, live site)
- **Quick Deploy**: Trigger deployments
- **QA Status**: Test pass rate per project

### Finance
- **Dashboard Widgets**:
  - Monthly revenue
  - Outstanding invoices
  - Upcoming payments
  - Cash flow chart
- **Invoice Generator**: Create, preview, send
- **Transaction Log**: All in/out

## Data Storage

Using JSON files in `/dashboard/data/`:
- `leads.json` - CRM contacts and pipeline
- `content.json` - Marketing content library
- `projects.json` - Development projects
- `invoices.json` - Finance records
- `agents.json` - AI agent configurations
- `activity.json` - Recent activity log

## Design System

- **Colors**: Indigo primary (#6366f1), department accent colors
- **Cards**: Gray-800 bg, rounded-xl, subtle border
- **Typography**: Inter font, clear hierarchy
- **Icons**: Font Awesome 6
- **Charts**: Chart.js
- **Responsive**: Mobile-first, collapsible sidebar

## Implementation Order

1. ✅ Base layout with new navigation
2. ✅ Main dashboard with widgets
3. ✅ AI Operations with interactive diagram
4. ✅ Sales CRM with pipeline
5. ✅ Marketing content library
6. ✅ Development projects
7. ✅ Finance overview
8. ✅ Data persistence layer
9. ✅ Polish and mobile optimization
