using System;
using System.Net.Http;
using System.Net.Sockets;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Microsoft.Win32;

namespace IPLoop
{
    /// <summary>
    /// IPLoop SDK for Windows - Residential Proxy Network
    /// </summary>
    public class IPLoopSDK
    {
        private static readonly Lazy<IPLoopSDK> _instance = new(() => new IPLoopSDK());
        public static IPLoopSDK Shared => _instance.Value;

        private string? _apiKey;
        private string _registrationUrl = "http://178.128.172.81:8001";
        private string _deviceId;
        private bool _isRunning;
        private Timer? _heartbeatTimer;
        private TcpListener? _proxyListener;
        private CancellationTokenSource? _cts;
        private readonly HttpClient _httpClient = new();
        
        private const string SdkVersion = "1.0.0";
        private const int HeartbeatIntervalMs = 30000;

        public event Action<SDKStatus>? OnStatusChange;
        public event Action<Exception>? OnError;

        public BandwidthStats Stats { get; private set; } = new();

        private IPLoopSDK()
        {
            _deviceId = GetOrCreateDeviceId();
        }

        /// <summary>
        /// Initialize the SDK with your API key
        /// </summary>
        public void Initialize(string apiKey)
        {
            _apiKey = apiKey;
            Log("SDK initialized");
        }

        /// <summary>
        /// Start the proxy service
        /// </summary>
        public async Task StartAsync()
        {
            if (string.IsNullOrEmpty(_apiKey))
                throw new InvalidOperationException("SDK not initialized. Call Initialize() first.");

            if (_isRunning) return;

            Log("Starting SDK...");
            OnStatusChange?.Invoke(SDKStatus.Connecting);

            try
            {
                // Register device
                await RegisterDeviceAsync();

                // Start proxy server
                StartProxyServer();

                // Start heartbeat
                StartHeartbeat();

                _isRunning = true;
                OnStatusChange?.Invoke(SDKStatus.Connected);
                Log("SDK started successfully");
            }
            catch (Exception ex)
            {
                OnStatusChange?.Invoke(SDKStatus.Error);
                OnError?.Invoke(ex);
                throw;
            }
        }

        /// <summary>
        /// Stop the proxy service
        /// </summary>
        public async Task StopAsync()
        {
            if (!_isRunning) return;

            Log("Stopping SDK...");
            OnStatusChange?.Invoke(SDKStatus.Disconnecting);

            _heartbeatTimer?.Dispose();
            _heartbeatTimer = null;

            _cts?.Cancel();
            _proxyListener?.Stop();
            _proxyListener = null;

            await UnregisterDeviceAsync();

            _isRunning = false;
            OnStatusChange?.Invoke(SDKStatus.Disconnected);
            Log("SDK stopped");
        }

        /// <summary>
        /// Check if SDK is running
        /// </summary>
        public bool IsActive => _isRunning;

        /// <summary>
        /// Set user consent for GDPR compliance
        /// </summary>
        public void SetUserConsent(bool consent)
        {
            try
            {
                using var key = Registry.CurrentUser.CreateSubKey(@"SOFTWARE\IPLoop");
                key?.SetValue("UserConsent", consent ? 1 : 0);
            }
            catch { }
        }

        /// <summary>
        /// Check if user has given consent
        /// </summary>
        public bool HasUserConsent()
        {
            try
            {
                using var key = Registry.CurrentUser.OpenSubKey(@"SOFTWARE\IPLoop");
                return key?.GetValue("UserConsent") is int val && val == 1;
            }
            catch { return false; }
        }

        private async Task RegisterDeviceAsync()
        {
            var body = new
            {
                device_id = _deviceId,
                device_type = "windows",
                sdk_version = SdkVersion,
                os_version = Environment.OSVersion.ToString(),
                device_model = Environment.MachineName,
                connection_type = "wifi"
            };

            var content = new StringContent(
                JsonSerializer.Serialize(body),
                Encoding.UTF8,
                "application/json");

            _httpClient.DefaultRequestHeaders.Clear();
            _httpClient.DefaultRequestHeaders.Add("X-API-Key", _apiKey);

            var response = await _httpClient.PostAsync($"{_registrationUrl}/register", content);
            
            if (!response.IsSuccessStatusCode)
                throw new Exception($"Registration failed: {response.StatusCode}");

            Log("Device registered");
        }

        private async Task UnregisterDeviceAsync()
        {
            try
            {
                var body = new { device_id = _deviceId };
                var content = new StringContent(
                    JsonSerializer.Serialize(body),
                    Encoding.UTF8,
                    "application/json");

                await _httpClient.PostAsync($"{_registrationUrl}/unregister", content);
            }
            catch { }
        }

        private void StartProxyServer()
        {
            _cts = new CancellationTokenSource();
            _proxyListener = new TcpListener(System.Net.IPAddress.Loopback, 0);
            _proxyListener.Start();

            Task.Run(async () =>
            {
                while (!_cts.Token.IsCancellationRequested)
                {
                    try
                    {
                        var client = await _proxyListener.AcceptTcpClientAsync(_cts.Token);
                        _ = HandleClientAsync(client);
                    }
                    catch (OperationCanceledException) { break; }
                    catch { }
                }
            });

            Log($"Proxy server started on port {((System.Net.IPEndPoint)_proxyListener.LocalEndpoint).Port}");
        }

        private async Task HandleClientAsync(TcpClient client)
        {
            Stats.TotalRequests++;
            // In production: implement full proxy handling
            await Task.Delay(100);
            client.Close();
        }

        private void StartHeartbeat()
        {
            _heartbeatTimer = new Timer(async _ =>
            {
                await SendHeartbeatAsync();
            }, null, HeartbeatIntervalMs, HeartbeatIntervalMs);
        }

        private async Task SendHeartbeatAsync()
        {
            try
            {
                var body = new
                {
                    device_id = _deviceId,
                    bandwidth_used_mb = Stats.TotalMB,
                    total_requests = Stats.TotalRequests
                };

                var content = new StringContent(
                    JsonSerializer.Serialize(body),
                    Encoding.UTF8,
                    "application/json");

                await _httpClient.PostAsync($"{_registrationUrl}/heartbeat", content);
            }
            catch { }
        }

        private string GetOrCreateDeviceId()
        {
            try
            {
                using var key = Registry.CurrentUser.CreateSubKey(@"SOFTWARE\IPLoop");
                var id = key?.GetValue("DeviceId") as string;
                
                if (string.IsNullOrEmpty(id))
                {
                    id = Guid.NewGuid().ToString();
                    key?.SetValue("DeviceId", id);
                }
                
                return id;
            }
            catch
            {
                return Guid.NewGuid().ToString();
            }
        }

        private void Log(string message)
        {
#if DEBUG
            Console.WriteLine($"[IPLoopSDK] {message}");
#endif
        }
    }

    public enum SDKStatus
    {
        Disconnected,
        Connecting,
        Connected,
        Disconnecting,
        Error
    }

    public class BandwidthStats
    {
        public long TotalBytes { get; set; }
        public int TotalRequests { get; set; }
        public double TotalMB => TotalBytes / 1048576.0;
    }
}
