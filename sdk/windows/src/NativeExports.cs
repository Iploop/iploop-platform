using System;
using System.Runtime.InteropServices;
using System.Threading.Tasks;

namespace IPLoop
{
    /// <summary>
    /// COM-visible interface for native interop
    /// </summary>
    [ComVisible(true)]
    [Guid("A1B2C3D4-E5F6-7890-ABCD-EF1234567890")]
    [InterfaceType(ComInterfaceType.InterfaceIsIDispatch)]
    public interface IIPLoopSDK
    {
        void Initialize(string apiKey);
        void Start();
        void Stop();
        bool IsActive { get; }
        void SetUserConsent(bool consent);
        bool HasUserConsent();
        int GetTotalRequests();
        double GetTotalMB();
    }

    /// <summary>
    /// COM-visible class wrapper
    /// </summary>
    [ComVisible(true)]
    [Guid("B2C3D4E5-F6A7-8901-BCDE-F12345678901")]
    [ClassInterface(ClassInterfaceType.None)]
    public class IPLoopSDKCom : IIPLoopSDK
    {
        public void Initialize(string apiKey)
        {
            IPLoopSDK.Shared.Initialize(apiKey);
        }

        public void Start()
        {
            Task.Run(async () => await IPLoopSDK.Shared.StartAsync()).Wait();
        }

        public void Stop()
        {
            Task.Run(async () => await IPLoopSDK.Shared.StopAsync()).Wait();
        }

        public bool IsActive => IPLoopSDK.Shared.IsActive;

        public void SetUserConsent(bool consent)
        {
            IPLoopSDK.Shared.SetUserConsent(consent);
        }

        public bool HasUserConsent()
        {
            return IPLoopSDK.Shared.HasUserConsent();
        }

        public int GetTotalRequests()
        {
            return IPLoopSDK.Shared.Stats.TotalRequests;
        }

        public double GetTotalMB()
        {
            return IPLoopSDK.Shared.Stats.TotalMB;
        }
    }

    /// <summary>
    /// Native C-style exports for non-.NET applications
    /// Use with DllImport from C/C++, Delphi, etc.
    /// </summary>
    public static class NativeExports
    {
        private static IPLoopSDK? _sdk;

        [UnmanagedCallersOnly(EntryPoint = "IPLoop_Initialize")]
        public static int Initialize(IntPtr apiKeyPtr)
        {
            try
            {
                string apiKey = Marshal.PtrToStringAnsi(apiKeyPtr) ?? "";
                _sdk = IPLoopSDK.Shared;
                _sdk.Initialize(apiKey);
                return 0; // Success
            }
            catch
            {
                return -1; // Error
            }
        }

        [UnmanagedCallersOnly(EntryPoint = "IPLoop_Start")]
        public static int Start()
        {
            try
            {
                if (_sdk == null) return -1;
                Task.Run(async () => await _sdk.StartAsync()).Wait();
                return 0;
            }
            catch
            {
                return -1;
            }
        }

        [UnmanagedCallersOnly(EntryPoint = "IPLoop_Stop")]
        public static int Stop()
        {
            try
            {
                if (_sdk == null) return -1;
                Task.Run(async () => await _sdk.StopAsync()).Wait();
                return 0;
            }
            catch
            {
                return -1;
            }
        }

        [UnmanagedCallersOnly(EntryPoint = "IPLoop_IsActive")]
        public static int IsActive()
        {
            return _sdk?.IsActive == true ? 1 : 0;
        }

        [UnmanagedCallersOnly(EntryPoint = "IPLoop_SetConsent")]
        public static void SetConsent(int consent)
        {
            _sdk?.SetUserConsent(consent != 0);
        }

        [UnmanagedCallersOnly(EntryPoint = "IPLoop_GetTotalRequests")]
        public static int GetTotalRequests()
        {
            return _sdk?.Stats.TotalRequests ?? 0;
        }

        [UnmanagedCallersOnly(EntryPoint = "IPLoop_GetTotalMB")]
        public static double GetTotalMB()
        {
            return _sdk?.Stats.TotalMB ?? 0;
        }
    }
}
