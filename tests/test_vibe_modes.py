"""Tests for the Vibe screen's run-mode switch (Normal / DSP / AUTO / AFK)."""

import asyncio
import tempfile
from pathlib import Path

import ashley.config as cfg
import ashley.tui.app as appmod
from ashley.tui.app import AshleyApp, _mode_flags


def _isolated_theme(tmp: Path):
    """Point theme storage at a temp dir and pre-configure a theme."""
    cfg.THEME_PATH = tmp / "theme.json"
    cfg.CONFIG_DIR = tmp
    appmod.theme_configured = cfg.theme_configured
    appmod.load_theme = cfg.load_theme
    appmod.save_theme = cfg.save_theme
    cfg.save_theme("dark", "blue")


def test_mode_flags_mapping():
    """Each run mode maps to the expected (dsp, auto, afk) booleans."""
    assert _mode_flags("normal") == (False, False, False)
    assert _mode_flags("dsp") == (True, False, False)
    assert _mode_flags("auto") == (False, True, False)
    assert _mode_flags("afk") == (False, False, True)


def _open_vibe(pilot, app):
    lv = app.screen.query_one("#feature-list")
    lv.index = [f["key"] for f in appmod.FEATURES].index("vibe")


def test_cycle_mode_advances_and_wraps():
    """Pressing 'm' cycles Normal → DSP → AUTO → AFK → Normal."""
    result: dict = {}

    async def scenario():
        with tempfile.TemporaryDirectory() as d:
            _isolated_theme(Path(d))
            app = AshleyApp()
            async with app.run_test(size=(120, 40)) as pilot:
                await pilot.pause()
                _open_vibe(pilot, app)
                await pilot.pause()
                await pilot.press("enter")
                await pilot.pause()
                screen = app.screen
                result["start"] = screen._run_mode
                seen = []
                for _ in range(4):
                    await pilot.press("m")
                    await pilot.pause()
                    seen.append(screen._run_mode)
                result["seen"] = seen
                result["exc"] = app._exception

    asyncio.run(scenario())
    assert result["exc"] is None
    assert result["start"] == "normal"
    assert result["seen"] == ["dsp", "auto", "afk", "normal"]


def test_clicking_chip_selects_mode():
    """Clicking a chip selects that mode and marks it selected."""
    result: dict = {}

    async def scenario():
        with tempfile.TemporaryDirectory() as d:
            _isolated_theme(Path(d))
            app = AshleyApp()
            async with app.run_test(size=(120, 40)) as pilot:
                await pilot.pause()
                _open_vibe(pilot, app)
                await pilot.pause()
                await pilot.press("enter")
                await pilot.pause()
                await pilot.click("#mode-afk")
                await pilot.pause()
                screen = app.screen
                result["mode"] = screen._run_mode
                chip = screen.query_one("#mode-afk")
                result["selected"] = chip.has_class("-selected")
                result["exc"] = app._exception

    asyncio.run(scenario())
    assert result["exc"] is None
    assert result["mode"] == "afk"
    assert result["selected"] is True
