"""Capture original Python/Textual layouts for Go visual regression tests.

Run with the development environment; never used by an installed Go binary.
The temporary HOME keeps personal settings, history and sessions out of fixtures.
"""

import asyncio
import os
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


async def capture():
    from ashley.tui.app import (
        AshleyApp,
        HistoryScreen,
        SessionsScreen,
        SettingsScreen,
        StatsScreen,
        VibeScreen,
    )
    from ashley.tui.create_app import CreateScreen

    directory = ROOT / "testdata/ui/python"
    directory.mkdir(parents=True, exist_ok=True)
    app = AshleyApp()
    async with app.run_test(size=(100, 30)) as pilot:
        for name, screen in (
            ("hub", None),
            ("vibe", VibeScreen),
            ("sessions", SessionsScreen),
            ("history", HistoryScreen),
            ("stats", StatsScreen),
            ("settings", SettingsScreen),
            ("create", CreateScreen),
        ):
            if screen:
                await app.push_screen(screen())
            await pilot.pause(0.35)
            rows = [
                "".join(segment.text for segment in strip).rstrip()
                for strip in app.screen._compositor.render_strips()
            ]
            if export := os.environ.get("ASHLEY_UI_EXPORT"):
                from rich.color import ColorSystem

                target = Path(export)
                target.mkdir(parents=True, exist_ok=True)
                ansi = "\n".join(
                    "".join(
                        segment.style.render(
                            segment.text, color_system=ColorSystem.TRUECOLOR
                        )
                        if segment.style
                        else segment.text
                        for segment in strip
                    )
                    for strip in app.screen._compositor.render_strips()
                )
                (target / f"{name}.ansi").write_text(ansi)
            # Header includes the version and live clock, tested separately in Go.
            (directory / f"{name}.txt").write_text("\n".join(rows[1:]) + "\n")
            if screen:
                app.pop_screen()


if __name__ == "__main__":
    with tempfile.TemporaryDirectory(prefix="ashley-ui-reference-") as home:
        os.environ["HOME"] = home
        os.environ["XDG_DATA_HOME"] = str(Path(home) / "data")
        config = Path(home) / ".ashley"
        config.mkdir()
        (config / "theme.json").write_text('{"mode":"dark","preset":"blue"}')
        asyncio.run(capture())
