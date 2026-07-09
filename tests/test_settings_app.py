"""Tests for the appearance settings / first-run wizard and stats screen."""

import asyncio
import tempfile
from pathlib import Path

import ashley.config as cfg
import ashley.tui.app as appmod
from ashley.tui.app import AshleyApp


def _isolated_theme(tmp: Path):
    """Point theme storage at a temp dir for both config and app modules."""
    cfg.THEME_PATH = tmp / "theme.json"
    cfg.CONFIG_DIR = tmp
    appmod.theme_configured = cfg.theme_configured
    appmod.load_theme = cfg.load_theme
    appmod.save_theme = cfg.save_theme


def test_first_launch_shows_setup_wizard():
    """With no saved theme, the app opens the setup wizard, not the hub."""
    seen: list[str] = []

    async def scenario():
        with tempfile.TemporaryDirectory() as d:
            _isolated_theme(Path(d))
            app = AshleyApp()
            async with app.run_test() as pilot:
                await pilot.pause()
                seen.append(type(app.screen).__name__)

    asyncio.run(scenario())
    assert seen == ["SettingsScreen"]


def test_configured_launch_shows_hub():
    """With a saved theme, the app skips the wizard and opens the hub."""
    seen: list[str] = []

    async def scenario():
        with tempfile.TemporaryDirectory() as d:
            _isolated_theme(Path(d))
            cfg.save_theme("dark", "blue")
            app = AshleyApp()
            async with app.run_test() as pilot:
                await pilot.pause()
                seen.append(type(app.screen).__name__)

    asyncio.run(scenario())
    assert seen == ["HubScreen"]


def test_wizard_saves_and_applies_dual_tone():
    """Selecting a dual-tone preset previews live and persists on done."""
    result: dict = {}

    async def scenario():
        with tempfile.TemporaryDirectory() as d:
            _isolated_theme(Path(d))
            app = AshleyApp()
            async with app.run_test(size=(100, 40)) as pilot:
                await pilot.pause()
                await pilot.click("#sw-ocean")
                await pilot.click("#mode-light")
                await pilot.pause()
                result["accent"] = str(app.get_css_variables().get("accent"))
                await pilot.click("#done-btn")
                await pilot.pause()
                result["screen"] = type(app.screen).__name__
                result["saved"] = cfg.load_theme()
                result["exc"] = app._exception

    asyncio.run(scenario())
    assert result["exc"] is None
    assert result["screen"] == "HubScreen"
    assert result["saved"] == {"mode": "light", "preset": "ocean"}
    # Ocean's accent is cyan (#39C5CF), distinct from its blue primary.
    assert result["accent"].lower().startswith("#39c")


def test_swatches_navigable_and_selectable_by_keyboard():
    """Arrow keys move focus across the grid and Enter selects — no mouse."""
    result: dict = {}

    async def scenario():
        with tempfile.TemporaryDirectory() as d:
            _isolated_theme(Path(d))
            cfg.save_theme("dark", "blue")
            app = AshleyApp()
            async with app.run_test(size=(120, 40)) as pilot:
                await pilot.pause()
                lv = app.screen.query_one("#feature-list")
                lv.index = [f["key"] for f in appmod.FEATURES].index("settings")
                await pilot.pause()
                await pilot.press("enter")
                await pilot.pause()
                # Focus starts on the saved colour (blue).
                result["start"] = app.focused.id
                await pilot.press("right")  # green
                await pilot.press("down")  # green(1) + 5 cols = ocean
                await pilot.pause()
                result["after_nav"] = app.focused.id
                await pilot.press("enter")  # select focused swatch
                await pilot.pause()
                result["saved"] = cfg.load_theme()["preset"]
                result["exc"] = app._exception

    asyncio.run(scenario())
    assert result["exc"] is None
    assert result["start"] == "sw-blue"
    assert result["after_nav"] == "sw-ocean"
    assert result["saved"] == "ocean"


def test_arrows_move_across_sections():
    """Arrow keys alone flow between mode, colours and the Done button."""
    result: dict = {}

    async def scenario():
        with tempfile.TemporaryDirectory() as d:
            _isolated_theme(Path(d))
            cfg.save_theme("dark", "blue")
            app = AshleyApp()
            async with app.run_test(size=(120, 40)) as pilot:
                await pilot.pause()
                lv = app.screen.query_one("#feature-list")
                lv.index = [f["key"] for f in appmod.FEATURES].index("settings")
                await pilot.pause()
                await pilot.press("enter")
                await pilot.pause()
                # From the selected swatch (blue, top row) up reaches the mode chips.
                result["start"] = app.focused.id
                await pilot.press("up")
                await pilot.pause()
                result["up_to_mode"] = app.focused.id
                # Right moves to the Light chip; Enter selects it.
                await pilot.press("right")
                await pilot.press("enter")
                await pilot.pause()
                result["mode"] = cfg.load_theme()["mode"]
                # Down travels back through the two colour rows to the Done button.
                await pilot.press("down")
                await pilot.press("down")
                await pilot.press("down")
                await pilot.pause()
                result["bottom"] = app.focused.id
                result["exc"] = app._exception

    asyncio.run(scenario())
    assert result["exc"] is None
    assert result["start"] == "sw-blue"
    assert result["up_to_mode"] == "mode-dark"
    assert result["mode"] == "light"
    assert result["bottom"] == "done-btn"


def test_stats_screen_opens_without_error():
    """The hub's Stats entry opens an in-TUI screen (no subprocess)."""
    result: dict = {}

    async def scenario():
        with tempfile.TemporaryDirectory() as d:
            _isolated_theme(Path(d))
            cfg.save_theme("dark", "blue")
            app = AshleyApp()
            async with app.run_test(size=(100, 40)) as pilot:
                await pilot.pause()
                lv = app.screen.query_one("#feature-list")
                lv.index = [f["key"] for f in appmod.FEATURES].index("stats")
                await pilot.pause()
                await pilot.press("enter")
                await pilot.pause()
                result["screen"] = type(app.screen).__name__
                result["exc"] = app._exception

    asyncio.run(scenario())
    assert result["exc"] is None
    assert result["screen"] == "StatsScreen"
