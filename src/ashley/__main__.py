"""Allow running ashley as `python -m ashley`."""

import sys


def main():
    mod = sys.argv[1] if len(sys.argv) > 1 else None
    if mod == "generate" or mod == "list":
        from ashley.generate import generate, list_skills

        if mod == "list":
            list_skills()
        else:
            generate()
    elif mod == "install":
        from ashley.install import install, uninstall

        if len(sys.argv) > 2 and sys.argv[2] == "uninstall":
            uninstall()
        else:
            install()
    elif mod == "update":
        from ashley.update import update

        branch = sys.argv[2] if len(sys.argv) > 2 else "main"
        update(branch)
    else:
        from ashley.cli import main as cli_main

        cli_main()


if __name__ == "__main__":
    main()
