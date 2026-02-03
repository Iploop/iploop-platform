package io.iploop.node

import android.app.Application
import dagger.hilt.android.HiltAndroidApp

@HiltAndroidApp
class IPLoopApplication : Application() {
    
    override fun onCreate() {
        super.onCreate()
        instance = this
    }
    
    companion object {
        lateinit var instance: IPLoopApplication
            private set
    }
}
