package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"
)

// Sender handles email sending via different providers
type Sender struct {
	provider    string
	apiKey      string
	fromEmail   string
	fromName    string
	templates   map[string]*template.Template
}

// NewSender creates a new email sender
func NewSender() *Sender {
	provider := os.Getenv("EMAIL_PROVIDER") // "resend", "sendgrid", "ses"
	if provider == "" {
		provider = "resend"
	}

	s := &Sender{
		provider:  provider,
		apiKey:    os.Getenv("EMAIL_API_KEY"),
		fromEmail: os.Getenv("EMAIL_FROM_ADDRESS"),
		fromName:  os.Getenv("EMAIL_FROM_NAME"),
		templates: make(map[string]*template.Template),
	}

	if s.fromEmail == "" {
		s.fromEmail = "noreply@iploop.io"
	}
	if s.fromName == "" {
		s.fromName = "IPLoop"
	}

	s.loadTemplates()
	return s
}

// Email represents an email message
type Email struct {
	To       string
	Subject  string
	Template string
	Data     map[string]interface{}
}

// Send sends an email
func (s *Sender) Send(email Email) error {
	// Render template
	html, err := s.renderTemplate(email.Template, email.Data)
	if err != nil {
		return fmt.Errorf("failed to render template: %v", err)
	}

	switch s.provider {
	case "resend":
		return s.sendViaResend(email.To, email.Subject, html)
	case "sendgrid":
		return s.sendViaSendGrid(email.To, email.Subject, html)
	default:
		return s.sendViaResend(email.To, email.Subject, html)
	}
}

func (s *Sender) sendViaResend(to, subject, html string) error {
	payload := map[string]interface{}{
		"from":    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		"to":      []string{to},
		"subject": subject,
		"html":    html,
	}

	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("email send failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (s *Sender) sendViaSendGrid(to, subject, html string) error {
	payload := map[string]interface{}{
		"personalizations": []map[string]interface{}{
			{
				"to": []map[string]string{
					{"email": to},
				},
			},
		},
		"from": map[string]string{
			"email": s.fromEmail,
			"name":  s.fromName,
		},
		"subject": subject,
		"content": []map[string]string{
			{
				"type":  "text/html",
				"value": html,
			},
		},
	}

	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("email send failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (s *Sender) loadTemplates() {
	// Templates are loaded inline for simplicity
	// In production, these would be in separate files
}

func (s *Sender) renderTemplate(name string, data map[string]interface{}) (string, error) {
	templates := map[string]string{
		"welcome": `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 10px 10px; }
        .button { display: inline-block; background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to IPLoop! 🎉</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>Thank you for signing up for IPLoop! We're excited to have you on board.</p>
            <p>Your account is now active and ready to use. Here's what you can do next:</p>
            <ul>
                <li>Generate your first API key</li>
                <li>Explore our documentation</li>
                <li>Make your first proxy request</li>
            </ul>
            <a href="{{.DashboardURL}}" class="button">Go to Dashboard</a>
            <p>If you have any questions, feel free to reach out to our support team.</p>
            <p>Best regards,<br>The IPLoop Team</p>
        </div>
        <div class="footer">
            <p>© 2024 IPLoop. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`,

		"quota_warning": `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #f59e0b; color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 10px 10px; }
        .stats { background: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .button { display: inline-block; background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>⚠️ Quota Warning</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>You've used <strong>{{.UsagePercent}}%</strong> of your monthly bandwidth quota.</p>
            <div class="stats">
                <p><strong>Used:</strong> {{.UsedGB}} GB</p>
                <p><strong>Limit:</strong> {{.LimitGB}} GB</p>
                <p><strong>Remaining:</strong> {{.RemainingGB}} GB</p>
            </div>
            <p>To avoid service interruption, consider upgrading your plan.</p>
            <a href="{{.UpgradeURL}}" class="button">Upgrade Now</a>
        </div>
        <div class="footer">
            <p>© 2024 IPLoop. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`,

		"payment_success": `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #10b981; color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 10px 10px; }
        .receipt { background: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>✅ Payment Received</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>Thank you for your payment! Your subscription is now active.</p>
            <div class="receipt">
                <p><strong>Amount:</strong> ${{.Amount}}</p>
                <p><strong>Plan:</strong> {{.PlanName}}</p>
                <p><strong>Date:</strong> {{.Date}}</p>
                <p><strong>Invoice #:</strong> {{.InvoiceID}}</p>
            </div>
            <p>Your next billing date is {{.NextBillingDate}}.</p>
        </div>
        <div class="footer">
            <p>© 2024 IPLoop. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`,

		"payment_failed": `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #ef4444; color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 10px 10px; }
        .button { display: inline-block; background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>❌ Payment Failed</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>We were unable to process your payment for your IPLoop subscription.</p>
            <p>Please update your payment method to avoid service interruption.</p>
            <a href="{{.UpdatePaymentURL}}" class="button">Update Payment Method</a>
            <p>If you have any questions, please contact our support team.</p>
        </div>
        <div class="footer">
            <p>© 2024 IPLoop. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`,

		"api_key_created": `
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #3b82f6; color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9fafb; padding: 30px; border-radius: 0 0 10px 10px; }
        .key-box { background: #1e293b; color: #10b981; padding: 15px; border-radius: 8px; font-family: monospace; word-break: break-all; margin: 20px 0; }
        .footer { text-align: center; color: #666; font-size: 12px; margin-top: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔑 New API Key Created</h1>
        </div>
        <div class="content">
            <p>Hi {{.Name}},</p>
            <p>A new API key has been created for your account:</p>
            <p><strong>Key Name:</strong> {{.KeyName}}</p>
            <div class="key-box">{{.APIKey}}</div>
            <p>⚠️ This is the only time you'll see this key. Please save it securely.</p>
            <p>If you didn't create this key, please contact support immediately.</p>
        </div>
        <div class="footer">
            <p>© 2024 IPLoop. All rights reserved.</p>
        </div>
    </div>
</body>
</html>`,
	}

	tmplStr, ok := templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}

	tmpl, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// SendWelcome sends a welcome email
func (s *Sender) SendWelcome(to, name string) error {
	return s.Send(Email{
		To:       to,
		Subject:  "Welcome to IPLoop! 🎉",
		Template: "welcome",
		Data: map[string]interface{}{
			"Name":         name,
			"DashboardURL": os.Getenv("DASHBOARD_URL") + "/dashboard",
		},
	})
}

// SendQuotaWarning sends a quota warning email
func (s *Sender) SendQuotaWarning(to, name string, usagePercent float64, usedGB, limitGB float64) error {
	return s.Send(Email{
		To:       to,
		Subject:  "⚠️ IPLoop Quota Warning - " + fmt.Sprintf("%.0f%%", usagePercent) + " Used",
		Template: "quota_warning",
		Data: map[string]interface{}{
			"Name":         name,
			"UsagePercent": fmt.Sprintf("%.0f", usagePercent),
			"UsedGB":       fmt.Sprintf("%.2f", usedGB),
			"LimitGB":      fmt.Sprintf("%.0f", limitGB),
			"RemainingGB":  fmt.Sprintf("%.2f", limitGB-usedGB),
			"UpgradeURL":   os.Getenv("DASHBOARD_URL") + "/billing",
		},
	})
}

// SendPaymentSuccess sends a payment success email
func (s *Sender) SendPaymentSuccess(to, name string, amount float64, planName, invoiceID, nextBillingDate string) error {
	return s.Send(Email{
		To:       to,
		Subject:  "✅ Payment Received - IPLoop",
		Template: "payment_success",
		Data: map[string]interface{}{
			"Name":            name,
			"Amount":          fmt.Sprintf("%.2f", amount),
			"PlanName":        planName,
			"InvoiceID":       invoiceID,
			"Date":            time.Now().Format("January 2, 2006"),
			"NextBillingDate": nextBillingDate,
		},
	})
}

// SendPaymentFailed sends a payment failed email
func (s *Sender) SendPaymentFailed(to, name string) error {
	return s.Send(Email{
		To:       to,
		Subject:  "❌ Payment Failed - Action Required",
		Template: "payment_failed",
		Data: map[string]interface{}{
			"Name":             name,
			"UpdatePaymentURL": os.Getenv("DASHBOARD_URL") + "/billing",
		},
	})
}

// SendAPIKeyCreated sends an API key created notification
func (s *Sender) SendAPIKeyCreated(to, name, keyName, apiKey string) error {
	return s.Send(Email{
		To:       to,
		Subject:  "🔑 New API Key Created - IPLoop",
		Template: "api_key_created",
		Data: map[string]interface{}{
			"Name":    name,
			"KeyName": keyName,
			"APIKey":  apiKey,
		},
	})
}
