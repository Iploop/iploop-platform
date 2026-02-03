import { useState } from 'react';
import { useRouter } from 'next/router';
import { register } from '../utils/api';
import Head from 'next/head';

export default function Signup() {
  const [formData, setFormData] = useState({
    email: '',
    password: '',
    confirmPassword: '',
    firstName: '',
    lastName: '',
    company: ''
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const router = useRouter();

  const handleChange = (e) => {
    setFormData({
      ...formData,
      [e.target.name]: e.target.value
    });
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    if (formData.password !== formData.confirmPassword) {
      setError('Passwords do not match');
      setLoading(false);
      return;
    }

    try {
      const response = await register({
        email: formData.email,
        password: formData.password,
        firstName: formData.firstName,
        lastName: formData.lastName,
        company: formData.company
      });
      
      if (response.user) {
        // Registration successful, redirect to login
        router.push('/login?message=Registration successful! Please log in.');
      } else {
        setError('Registration failed. Please try again.');
      }
    } catch (err) {
      setError(err.message || 'Registration failed. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  const inputStyle = {
    width: '100%',
    padding: '8px 12px',
    backgroundColor: '#374151',
    border: '1px solid #4b5563',
    borderRadius: '6px',
    color: 'white',
    fontSize: '14px'
  };

  const labelStyle = {
    display: 'block',
    fontSize: '14px',
    fontWeight: '500',
    color: '#d1d5db',
    marginBottom: '4px'
  };

  return (
    <>
      <Head>
        <title>Sign Up - IPLoop</title>
      </Head>
      <div style={{
        minHeight: '100vh',
        backgroundColor: '#111827',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '48px 16px'
      }}>
        <div style={{ maxWidth: '400px', width: '100%' }}>
          <div style={{ textAlign: 'center', marginBottom: '32px' }}>
            <h1 style={{ fontSize: '32px', fontWeight: 'bold', color: '#3b82f6', marginBottom: '24px' }}>
              IPLoop
            </h1>
            <h2 style={{ fontSize: '24px', fontWeight: 'bold', color: 'white', marginBottom: '8px' }}>
              Create your account
            </h2>
            <p style={{ color: '#9ca3af', fontSize: '14px' }}>
              Or{' '}
              <a
                href="/login"
                style={{ color: '#3b82f6', textDecoration: 'none' }}
              >
                sign in to existing account
              </a>
            </p>
          </div>
          
          <form onSubmit={handleSubmit}>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '16px' }}>
              <div>
                <label htmlFor="firstName" style={labelStyle}>
                  First Name
                </label>
                <input
                  id="firstName"
                  name="firstName"
                  type="text"
                  required
                  style={inputStyle}
                  placeholder="First name"
                  value={formData.firstName}
                  onChange={handleChange}
                />
              </div>
              <div>
                <label htmlFor="lastName" style={labelStyle}>
                  Last Name
                </label>
                <input
                  id="lastName"
                  name="lastName"
                  type="text"
                  required
                  style={inputStyle}
                  placeholder="Last name"
                  value={formData.lastName}
                  onChange={handleChange}
                />
              </div>
            </div>

            <div style={{ marginBottom: '16px' }}>
              <label htmlFor="email" style={labelStyle}>
                Email Address
              </label>
              <input
                id="email"
                name="email"
                type="email"
                autoComplete="email"
                required
                style={inputStyle}
                placeholder="Email address"
                value={formData.email}
                onChange={handleChange}
              />
            </div>

            <div style={{ marginBottom: '16px' }}>
              <label htmlFor="company" style={labelStyle}>
                Company (Optional)
              </label>
              <input
                id="company"
                name="company"
                type="text"
                style={inputStyle}
                placeholder="Company name"
                value={formData.company}
                onChange={handleChange}
              />
            </div>

            <div style={{ marginBottom: '16px' }}>
              <label htmlFor="password" style={labelStyle}>
                Password
              </label>
              <input
                id="password"
                name="password"
                type="password"
                autoComplete="new-password"
                required
                style={inputStyle}
                placeholder="Password"
                value={formData.password}
                onChange={handleChange}
              />
            </div>

            <div style={{ marginBottom: '16px' }}>
              <label htmlFor="confirmPassword" style={labelStyle}>
                Confirm Password
              </label>
              <input
                id="confirmPassword"
                name="confirmPassword"
                type="password"
                autoComplete="new-password"
                required
                style={inputStyle}
                placeholder="Confirm password"
                value={formData.confirmPassword}
                onChange={handleChange}
              />
            </div>

            {error && (
              <div style={{
                backgroundColor: '#7f1d1d',
                border: '1px solid #dc2626',
                color: '#fecaca',
                padding: '12px',
                borderRadius: '8px',
                marginBottom: '16px'
              }}>
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              style={{
                width: '100%',
                padding: '12px',
                backgroundColor: '#3b82f6',
                color: 'white',
                border: 'none',
                borderRadius: '8px',
                fontSize: '16px',
                fontWeight: '500',
                cursor: loading ? 'not-allowed' : 'pointer',
                opacity: loading ? 0.5 : 1
              }}
            >
              {loading ? 'Creating account...' : 'Sign up'}
            </button>
          </form>
        </div>
      </div>
    </>
  );
}