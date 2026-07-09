"""Shared Textual styling and effects for Ashley's TUIs.

Centralizes the visual language (accent highlights, understated chrome)
so every screen looks consistent, and provides small animation helpers
used on mount.
"""

from textual.widget import Widget

# Base CSS applied app-wide. Screen- and app-level CSS is additive, so each
# view only needs to declare its own layout on top of these shared rules.
#
# NOTE: deliberately no `transition` on the list cursor — animating the
# highlight background leaves a trail of half-faded rows as the cursor moves.
BASE_CSS = """
Screen {
    background: $surface;
}

/* Understated chrome: accent lives in the content, not the frame. */
Header {
    background: $panel;
    color: $accent;
    text-style: bold;
}

HeaderIcon {
    color: $accent;
}

Footer {
    background: $panel;
}

Footer > .footer--key {
    background: $surface-lighten-1;
    color: $accent;
    text-style: bold;
}

ListView {
    background: transparent;
    scrollbar-size-vertical: 1;
    scrollbar-color: $accent 50%;
}

ListView > ListItem {
    width: 1fr;
    padding: 0 1;
    background: transparent;
    color: $text;
}

/* Blurred list: a dim accent bar marks the cursor row. */
ListView > ListItem.-highlight {
    background: $accent 30%;
}

/* Focused list: a solid, full-width accent bar with contrasting text. */
ListView:focus > ListItem.-highlight {
    background: $accent;
    color: $surface;
    text-style: bold;
}

Input {
    border: tall $surface-lighten-2;
}

Input:focus {
    border: tall $accent;
}
"""


def apply_theme(app) -> None:
    """Build and activate the user's saved appearance theme on an app.

    Registers a custom Textual theme named ``"ashley"`` derived from the
    saved primary/accent preset and dark/light mode, then activates it.
    Safe to call again to re-apply after settings change.

    Args:
        app: The running Textual ``App`` to theme.
    """
    from textual.theme import Theme

    from ashley.config import THEME_PRESETS, load_theme

    settings = load_theme()
    preset = THEME_PRESETS.get(settings["preset"], THEME_PRESETS["blue"])
    theme = Theme(
        name="ashley",
        primary=preset["primary"],
        accent=preset["accent"],
        dark=(settings["mode"] == "dark"),
    )
    app.register_theme(theme)
    # Force a refresh even when the theme name is unchanged (live preview).
    if app.theme == "ashley":
        app.theme = "textual-dark"
    app.theme = "ashley"


def fade_in(widget: Widget, duration: float = 0.22) -> None:
    """Fade a widget from transparent to fully opaque.

    A no-op-safe cosmetic touch used on mount to soften screen entrances.

    Args:
        widget: The widget to animate.
        duration: Animation length in seconds.
    """
    widget.styles.opacity = 0.0
    widget.styles.animate("opacity", value=1.0, duration=duration)
