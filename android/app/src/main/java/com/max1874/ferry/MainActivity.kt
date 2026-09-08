package com.max1874.ferry

import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.enableEdgeToEdge
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider

class MainActivity : ComponentActivity() {
    private val model: FerryViewModel by viewModels {
        val content = AndroidContentStore(contentResolver)
        val applicationContext = applicationContext
        val deviceName = listOf(Build.MANUFACTURER, Build.MODEL)
            .map(String::trim).filter(String::isNotEmpty).distinct().joinToString(" ")
            .ifBlank { "Android" }
        object : ViewModelProvider.Factory {
            @Suppress("UNCHECKED_CAST")
            override fun <T : ViewModel> create(modelClass: Class<T>): T {
                return FerryViewModel(
                    service = FerryClient(content),
                    credentials = AndroidCredentialStore(applicationContext),
                    settings = AndroidSettingsStore(applicationContext),
                    contentStore = content,
                    defaultDeviceName = deviceName,
                ) as T
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent { FerryApp(model) }
        model.start()
    }

    override fun onStart() {
        super.onStart()
        model.setActive(true)
    }

    override fun onStop() {
        if (!isChangingConfigurations) model.setActive(false)
        super.onStop()
    }
}
