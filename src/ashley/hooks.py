"""Ashley Hook Runner.

Executes pre/post hooks around skill invocations. Hooks are shell
commands that run in the skill's working directory with environment
variables providing context:

    ASHLEY_SKILL       — the skill name (e.g., "feat")
    ASHLEY_QUESTION    — the user's question
    ASHLEY_EXIT_CODE   — claude's exit code (after_run/on_error only)
    ASHLEY_PERMISSION  — permission mode (default/auto/dsp/afk)
    ASHLEY_SESSION_ID  — session ID (detached mode only)
"""

import os
import subprocess

from ashley.config import HookConfig


def _build_env(
    skill: str,
    question: str = "",
    exit_code: int | None = None,
    permission: str = "default",
    session_id: str = "",
) -> dict[str, str]:
    """Build the environment dict for hook execution."""
    env = {
        **os.environ,
        "ASHLEY_SKILL": skill,
        "ASHLEY_QUESTION": question,
        "ASHLEY_PERMISSION": permission,
    }
    if exit_code is not None:
        env["ASHLEY_EXIT_CODE"] = str(exit_code)
    if session_id:
        env["ASHLEY_SESSION_ID"] = session_id
    return env


def run_hooks(
    commands: list[str],
    cwd: str,
    skill: str,
    question: str = "",
    exit_code: int | None = None,
    permission: str = "default",
    session_id: str = "",
) -> bool:
    """Execute a list of hook commands sequentially.

    Returns True if all commands succeeded, False if any failed.
    """
    if not commands:
        return True

    env = _build_env(skill, question, exit_code, permission, session_id)

    for cmd in commands:
        try:
            result = subprocess.run(
                cmd,
                shell=True,
                cwd=cwd,
                env=env,
                timeout=60,
            )
            if result.returncode != 0:
                return False
        except subprocess.TimeoutExpired:
            return False
        except Exception:
            return False

    return True


def run_before_hooks(
    hooks: HookConfig,
    skill: str,
    question: str,
    cwd: str,
    permission: str = "default",
) -> bool:
    """Run before_run hooks. Returns False if the run should be aborted."""
    return run_hooks(
        hooks.before_run,
        cwd=cwd,
        skill=skill,
        question=question,
        permission=permission,
    )


def run_after_hooks(
    hooks: HookConfig,
    skill: str,
    question: str,
    cwd: str,
    exit_code: int,
    permission: str = "default",
    session_id: str = "",
) -> None:
    """Run after_run hooks, and on_error hooks if exit_code != 0."""
    run_hooks(
        hooks.after_run,
        cwd=cwd,
        skill=skill,
        question=question,
        exit_code=exit_code,
        permission=permission,
        session_id=session_id,
    )

    if exit_code != 0:
        run_hooks(
            hooks.on_error,
            cwd=cwd,
            skill=skill,
            question=question,
            exit_code=exit_code,
            permission=permission,
            session_id=session_id,
        )
