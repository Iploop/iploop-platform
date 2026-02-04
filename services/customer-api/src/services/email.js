const nodemailer = require('nodemailer');

// Email configuration from environment
const EMAIL_PROVIDER = process.env.EMAIL_PROVIDER || 'smtp'; // 'smtp' | 'sendgrid'
const SMTP_HOST = process.env.SMTP_HOST || 'smtp.sendgrid.net';
const SMTP_PORT = process.env.SMTP_PORT || 587;
const SMTP_USER = process.env.SMTP_USER || 'apikey';
const SMTP_PASS = process.env.SMTP_PASS || process.env.SENDGRID_API_KEY;
const EMAIL_FROM = process.env.EMAIL_FROM || 'noreply@iploop.io';
const EMAIL_FROM_NAME = process.env.EMAIL_FROM_NAME || 'IPLoop';

let transporter = null;

// Initialize transporter
function getTransporter() {
  if (!transporter && SMTP_PASS) {
    transporter = nodemailer.createTransport({
      host: SMTP_HOST,
      port: SMTP_PORT,
      secure: SMTP_PORT === 465,
      auth: {
        user: SMTP_USER,
        pass: SMTP_PASS
      }
    });
  }
  return transporter;
}

// Send email
async function sendEmail({ to, subject, html, text }) {
  const transport = getTransporter();
  
  if (!transport) {
    console.log('[EMAIL] No email configured, logging instead:', { to, subject });
    return { success: true, mock: true };
  }

  try {
    const result = await transport.sendMail({
      from: `"${EMAIL_FROM_NAME}" <${EMAIL_FROM}>`,
      to,
      subject,
      html,
      text: text || html.replace(/<[^>]*>/g, '')
    });
    console.log('[EMAIL] Sent:', { to, subject, messageId: result.messageId });
    return { success: true, messageId: result.messageId };
  } catch (error) {
    console.error('[EMAIL] Failed:', error.message);
    throw error;
  }
}

// Email templates
const templates = {
  welcomeEmail: (user) => ({
    subject: 'Welcome to IPLoop!',
    html: `
      <h1>Welcome to IPLoop, ${user.firstName}!</h1>
      <p>Your account has been created successfully.</p>
      <p>Get started by:</p>
      <ol>
        <li>Creating an API key in your dashboard</li>
        <li>Connecting your first proxy node</li>
        <li>Making your first proxy request</li>
      </ol>
      <p><a href="https://dashboard.iploop.io">Go to Dashboard</a></p>
      <p>Need help? Reply to this email or check our <a href="https://docs.iploop.io">documentation</a>.</p>
      <p>— The IPLoop Team</p>
    `
  }),

  passwordReset: (data) => ({
    subject: 'Reset Your IPLoop Password',
    html: `
      <h1>Password Reset Request</h1>
      <p>Hi ${data.firstName},</p>
      <p>We received a request to reset your password. Click the link below to set a new password:</p>
      <p><a href="${data.resetUrl}" style="background:#0070f3;color:white;padding:12px 24px;text-decoration:none;border-radius:6px;display:inline-block;">Reset Password</a></p>
      <p>This link expires in 1 hour.</p>
      <p>If you didn't request this, you can safely ignore this email.</p>
      <p>— The IPLoop Team</p>
    `
  }),

  quotaWarning: (user, usagePercent) => ({
    subject: `IPLoop: You've used ${usagePercent}% of your quota`,
    html: `
      <h1>Quota Warning</h1>
      <p>Hi ${user.firstName},</p>
      <p>You've used <strong>${usagePercent}%</strong> of your monthly data quota.</p>
      <p>Consider upgrading your plan to avoid service interruption.</p>
      <p><a href="https://dashboard.iploop.io/billing">Manage Subscription</a></p>
      <p>— The IPLoop Team</p>
    `
  }),

  apiKeyCreated: (user, keyName) => ({
    subject: 'New API Key Created',
    html: `
      <h1>New API Key Created</h1>
      <p>Hi ${user.firstName},</p>
      <p>A new API key "<strong>${keyName}</strong>" was created for your account.</p>
      <p>If you didn't create this key, please secure your account immediately.</p>
      <p><a href="https://dashboard.iploop.io/api-keys">View API Keys</a></p>
      <p>— The IPLoop Team</p>
    `
  })
};

// Send templated email
async function sendTemplateEmail(templateName, to, data) {
  const template = templates[templateName];
  if (!template) {
    throw new Error(`Unknown email template: ${templateName}`);
  }
  
  const { subject, html } = template(data);
  return sendEmail({ to, subject, html });
}

module.exports = {
  sendEmail,
  sendTemplateEmail,
  templates
};
