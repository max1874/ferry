package com.max1874.ferry
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.scale
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

private val devicePaths = mapOf(
    DeviceKind.IPHONE to listOf("M6 5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2V5", "M11 4h2", "M12 17v.01"),
    DeviceKind.IPAD to listOf("M5 4a1 1 0 0 1 1-1h12a1 1 0 0 1 1 1v16a1 1 0 0 1-1 1H6a1 1 0 0 1-1-1V4", "M11 17a1 1 0 1 0 2 0 1 1 0 0 0-2 0"),
    DeviceKind.MAC to listOf("M3 5a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V5", "M7 20h10", "M9 16v4", "M15 16v4"),
    DeviceKind.ANDROID to listOf("M4 10v6", "M20 10v6", "M7 9h10v8a1 1 0 0 1-1 1H8a1 1 0 0 1-1-1V9a5 5 0 0 1 10 0", "M8 3l1 2", "M16 3l-1 2", "M9 18v3", "M15 18v3"),
    DeviceKind.WINDOWS to listOf("M17.8 20l-12-1.5A2 2 0 0 1 4 16.6V7.4a2 2 0 0 1 1.8-1.9l12-1.5A2 2 0 0 1 20 5.9V18a2 2 0 0 1-2.2 1.9V20", "M12 5v14", "M4 12h16"),
    DeviceKind.BROWSER to listOf("M4 8h16", "M4 6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6", "M8 4v4"),
)

@Composable
fun DeviceIcon(kind: DeviceKind, color: Color, modifier: Modifier = Modifier, size: Dp = 20.dp) {
    val paths = remember(kind) { devicePaths.getValue(kind).map { PathParser().parsePathString(it).toPath() } }
    Canvas(modifier.size(size)) {
        val factor = this.size.minDimension / 24f
        scale(factor, pivot = androidx.compose.ui.geometry.Offset.Zero) {
            paths.forEach { path: Path ->
                drawPath(
                    path = path,
                    color = color,
                    style = Stroke(width = 1.8f, cap = StrokeCap.Round, join = StrokeJoin.Round),
                )
            }
        }
    }
}
