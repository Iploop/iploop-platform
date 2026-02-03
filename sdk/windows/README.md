# IPLoop Windows SDK (DLL)

Enable your Windows application to participate in the IPLoop residential proxy network.

**Output:** `IPLoopSDK.dll` - works with .NET, C++, Delphi, VB6, and more.

## Requirements

- Windows 10/11
- .NET 6.0+ (for .NET apps) OR .NET Framework 4.8 (for legacy apps)

## Installation

### NuGet Package

```bash
dotnet add package IPLoopSDK
```

### Build DLL from source

```bash
# Build for all frameworks
dotnet build -c Release

# Output locations:
# bin/Release/net6.0/IPLoopSDK.dll        (.NET 6+)
# bin/Release/net48/IPLoopSDK.dll         (.NET Framework)
# bin/Release/netstandard2.0/IPLoopSDK.dll (Universal)
```

### Register COM DLL (for VB6, Delphi, etc.)

```bash
# Register
regsvr32 IPLoopSDK.comhost.dll

# Unregister
regsvr32 /u IPLoopSDK.comhost.dll
```

## Quick Start

### Console Application

```csharp
using IPLoop;

class Program
{
    static async Task Main(string[] args)
    {
        var sdk = IPLoopSDK.Shared;
        
        // Initialize
        sdk.Initialize("your_api_key");
        
        // Set consent
        sdk.SetUserConsent(true);
        
        // Monitor status
        sdk.OnStatusChange += status => 
            Console.WriteLine($"Status: {status}");
        
        // Start
        await sdk.StartAsync();
        
        Console.WriteLine("Press Enter to stop...");
        Console.ReadLine();
        
        // Stop
        await sdk.StopAsync();
    }
}
```

### Windows Service

```csharp
using IPLoop;
using Microsoft.Extensions.Hosting;

public class IPLoopService : BackgroundService
{
    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        var sdk = IPLoopSDK.Shared;
        sdk.Initialize(Environment.GetEnvironmentVariable("IPLOOP_API_KEY"));
        sdk.SetUserConsent(true);
        
        await sdk.StartAsync();
        
        while (!stoppingToken.IsCancellationRequested)
        {
            await Task.Delay(60000, stoppingToken);
            var stats = sdk.Stats;
            Console.WriteLine($"Stats: {stats.TotalRequests} requests, {stats.TotalMB:F2} MB");
        }
        
        await sdk.StopAsync();
    }
}

// In Program.cs:
Host.CreateDefaultBuilder(args)
    .UseWindowsService()
    .ConfigureServices(services => services.AddHostedService<IPLoopService>())
    .Build()
    .Run();
```

### WPF/WinForms Integration

```csharp
public partial class MainWindow : Window
{
    private readonly IPLoopSDK _sdk = IPLoopSDK.Shared;

    public MainWindow()
    {
        InitializeComponent();
        
        _sdk.Initialize("your_api_key");
        _sdk.OnStatusChange += status => 
            Dispatcher.Invoke(() => StatusLabel.Content = status.ToString());
    }

    private async void StartButton_Click(object sender, RoutedEventArgs e)
    {
        if (_sdk.HasUserConsent())
        {
            await _sdk.StartAsync();
        }
        else
        {
            // Show consent dialog
        }
    }

    private async void StopButton_Click(object sender, RoutedEventArgs e)
    {
        await _sdk.StopAsync();
    }
}
```

## API Reference

### Initialize
```csharp
IPLoopSDK.Shared.Initialize(string apiKey);
```

### Start/Stop
```csharp
await IPLoopSDK.Shared.StartAsync();
await IPLoopSDK.Shared.StopAsync();
```

### Status
```csharp
bool isActive = IPLoopSDK.Shared.IsActive;

IPLoopSDK.Shared.OnStatusChange += status => { ... };
IPLoopSDK.Shared.OnError += ex => { ... };
```

### Consent (GDPR)
```csharp
IPLoopSDK.Shared.SetUserConsent(true);
bool hasConsent = IPLoopSDK.Shared.HasUserConsent();
```

### Statistics
```csharp
var stats = IPLoopSDK.Shared.Stats;
Console.WriteLine($"Requests: {stats.TotalRequests}");
Console.WriteLine($"Bandwidth: {stats.TotalMB} MB");
```

## Install as Windows Service

```bash
# Build
dotnet publish -c Release -r win-x64 --self-contained

# Install service
sc create IPLoopService binPath="C:\path\to\IPLoopService.exe"
sc start IPLoopService
```

## Usage from Other Languages

### C/C++ (Native DLL)

```cpp
// Load DLL
typedef int (*InitFunc)(const char*);
typedef int (*StartFunc)();
typedef int (*StopFunc)();

HMODULE dll = LoadLibrary("IPLoopSDK.dll");
InitFunc init = (InitFunc)GetProcAddress(dll, "IPLoop_Initialize");
StartFunc start = (StartFunc)GetProcAddress(dll, "IPLoop_Start");
StopFunc stop = (StopFunc)GetProcAddress(dll, "IPLoop_Stop");

init("your_api_key");
start();
// ... 
stop();
FreeLibrary(dll);
```

### Delphi/Pascal

```pascal
function IPLoop_Initialize(apiKey: PAnsiChar): Integer; stdcall; external 'IPLoopSDK.dll';
function IPLoop_Start: Integer; stdcall; external 'IPLoopSDK.dll';
function IPLoop_Stop: Integer; stdcall; external 'IPLoopSDK.dll';

procedure StartIPLoop;
begin
  IPLoop_Initialize('your_api_key');
  IPLoop_Start;
end;
```

### VB6 / VBA (COM)

```vb
' Add reference to IPLoopSDK
Dim sdk As New IPLoopSDKCom

sdk.Initialize "your_api_key"
sdk.SetUserConsent True
sdk.Start

' Later...
MsgBox "Requests: " & sdk.GetTotalRequests
sdk.Stop
```

### Python (ctypes)

```python
from ctypes import cdll, c_char_p, c_int

dll = cdll.LoadLibrary("IPLoopSDK.dll")

dll.IPLoop_Initialize.argtypes = [c_char_p]
dll.IPLoop_Initialize.restype = c_int

dll.IPLoop_Initialize(b"your_api_key")
dll.IPLoop_Start()

# Later...
print(f"Requests: {dll.IPLoop_GetTotalRequests()}")
dll.IPLoop_Stop()
```

## DLL Exports

| Function | Description | Return |
|----------|-------------|--------|
| `IPLoop_Initialize(char* apiKey)` | Initialize SDK | 0=success, -1=error |
| `IPLoop_Start()` | Start proxy service | 0=success, -1=error |
| `IPLoop_Stop()` | Stop proxy service | 0=success, -1=error |
| `IPLoop_IsActive()` | Check if running | 1=active, 0=inactive |
| `IPLoop_SetConsent(int)` | Set user consent | void |
| `IPLoop_GetTotalRequests()` | Get request count | int |
| `IPLoop_GetTotalMB()` | Get bandwidth used | double |

## Support

- Email: support@iploop.io
- Docs: https://docs.iploop.io
