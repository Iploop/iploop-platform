const Stripe = require('stripe');

const STRIPE_SECRET_KEY = process.env.STRIPE_SECRET_KEY;
const STRIPE_WEBHOOK_SECRET = process.env.STRIPE_WEBHOOK_SECRET;

let stripe = null;

function getStripe() {
  if (!stripe && STRIPE_SECRET_KEY) {
    stripe = new Stripe(STRIPE_SECRET_KEY, {
      apiVersion: '2023-10-16'
    });
  }
  return stripe;
}

// Plan configuration
const PLANS = {
  starter: {
    name: 'Starter',
    priceId: process.env.STRIPE_PRICE_STARTER, // Set in env
    gbIncluded: 5,
    pricePerGb: 10,
    features: ['5 GB included', 'HTTP & SOCKS5', 'API access', 'Email support']
  },
  pro: {
    name: 'Pro',
    priceId: process.env.STRIPE_PRICE_PRO,
    gbIncluded: 50,
    pricePerGb: 8,
    features: ['50 GB included', 'HTTP & SOCKS5', 'API access', 'Priority support', 'Webhooks', 'Country targeting']
  },
  enterprise: {
    name: 'Enterprise',
    priceId: process.env.STRIPE_PRICE_ENTERPRISE,
    gbIncluded: 500,
    pricePerGb: 5,
    features: ['500 GB included', 'HTTP & SOCKS5', 'API access', 'Dedicated support', 'Webhooks', 'All countries', 'Custom SLA']
  }
};

// Create checkout session
async function createCheckoutSession(userId, userEmail, planKey, successUrl, cancelUrl) {
  const s = getStripe();
  if (!s) {
    throw new Error('Stripe not configured');
  }

  const plan = PLANS[planKey];
  if (!plan || !plan.priceId) {
    throw new Error(`Invalid plan: ${planKey}`);
  }

  const session = await s.checkout.sessions.create({
    customer_email: userEmail,
    payment_method_types: ['card'],
    line_items: [{
      price: plan.priceId,
      quantity: 1
    }],
    mode: 'subscription',
    success_url: successUrl || 'https://dashboard.iploop.io/billing?success=true',
    cancel_url: cancelUrl || 'https://dashboard.iploop.io/billing?canceled=true',
    metadata: {
      userId,
      planKey
    }
  });

  return session;
}

// Create customer portal session
async function createPortalSession(customerId, returnUrl) {
  const s = getStripe();
  if (!s) {
    throw new Error('Stripe not configured');
  }

  const session = await s.billingPortal.sessions.create({
    customer: customerId,
    return_url: returnUrl || 'https://dashboard.iploop.io/billing'
  });

  return session;
}

// Verify webhook signature
function constructWebhookEvent(payload, signature) {
  const s = getStripe();
  if (!s || !STRIPE_WEBHOOK_SECRET) {
    throw new Error('Stripe webhook not configured');
  }

  return s.webhooks.constructEvent(payload, signature, STRIPE_WEBHOOK_SECRET);
}

// Handle webhook events
async function handleWebhookEvent(event, db) {
  switch (event.type) {
    case 'checkout.session.completed': {
      const session = event.data.object;
      const { userId, planKey } = session.metadata;
      
      // Update user's plan in database
      console.log(`[STRIPE] Checkout completed: user=${userId}, plan=${planKey}`);
      // TODO: Update user_plans table
      break;
    }
    
    case 'customer.subscription.updated': {
      const subscription = event.data.object;
      console.log(`[STRIPE] Subscription updated: ${subscription.id}, status=${subscription.status}`);
      // TODO: Update subscription status in database
      break;
    }
    
    case 'customer.subscription.deleted': {
      const subscription = event.data.object;
      console.log(`[STRIPE] Subscription canceled: ${subscription.id}`);
      // TODO: Downgrade user to free plan
      break;
    }
    
    case 'invoice.payment_succeeded': {
      const invoice = event.data.object;
      console.log(`[STRIPE] Payment succeeded: ${invoice.id}, amount=${invoice.amount_paid}`);
      break;
    }
    
    case 'invoice.payment_failed': {
      const invoice = event.data.object;
      console.log(`[STRIPE] Payment failed: ${invoice.id}`);
      // TODO: Send email notification
      break;
    }
    
    default:
      console.log(`[STRIPE] Unhandled event type: ${event.type}`);
  }
}

module.exports = {
  getStripe,
  PLANS,
  createCheckoutSession,
  createPortalSession,
  constructWebhookEvent,
  handleWebhookEvent
};
