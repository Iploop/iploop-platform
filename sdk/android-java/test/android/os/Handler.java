package android.os;

public class Handler {
    public Handler(Looper looper) {}
    
    public void post(Runnable r) { 
        // Execute immediately in test environment
        r.run(); 
    }
}