# Python Technology Stack

Use this stack unless the user specifies otherwise.

- **Package management:** Astral/uv for deps, venvs, scripts. `pyproject.toml` as canonical config.
- **Runtime:** asyncio + uvloop for event loop. `async`/`await` for I/O. `aiomultiprocess`/`multiprocessing` for CPU-bound.
- **Web backend:** FastAPI + Uvicorn + Starlette. Async route handlers exclusively.
- **Caching:** aiocache for async function-level caching.
- **File I/O:** aiofiles for non-blocking read/write.
- **Data modeling:** DataModel (Pydantic `BaseModel` variant from `res/code/python/data_model.py`). Key methods: `get_empty_obj()`, `get_keys()`, `from_dict()`, `json()` (with optional shrink/exclude-None). Hashable and comparable by JSON repr.

## Ruff Config
```toml
[tool.ruff]
target-version = "py313"
[tool.ruff.lint]
extend-select = ["I"]
[tool.ruff.lint.isort]
known-first-party = ["<project_name>"]
```
