import functools
import json
from typing import Any, Dict, Self

from pydantic import BaseModel


class DataModel(BaseModel):
    """Base data model extending Pydantic's BaseModel with convenience methods.

    Provides cached empty-object construction, dictionary-based instantiation,
    JSON serialization with optional field exclusion, and value-based hashing
    and equality for use in sets and comparisons.
    """

    @classmethod
    @functools.cache
    def get_empty_obj(cls) -> Self:
        """Return a cached empty instance of this model.

        Subclasses must override this method to provide a default instance
        with all fields populated (typically with zero-values or None).

        Returns:
            A default instance of the model.

        Raises:
            NotImplementedError: If the subclass has not implemented this method.
        """
        raise NotImplementedError(
            f"get_empty_obj for {cls.__name__} is not implemented"
        )

    @classmethod
    @functools.cache
    def get_keys(cls) -> Dict[str, Any]:
        """Return a cached mapping of field names to their default values.

        Derives keys from the empty object returned by ``get_empty_obj()``.

        Returns:
            A dictionary mapping field names to their default serialized values.
        """
        return {k: v for k, v in cls.get_empty_obj().model_dump().items()}

    @classmethod
    def from_dict(cls, input: Dict[str, Any]) -> Self:
        """Construct an instance from a dictionary, picking only known fields.

        Args:
            input: A dictionary containing field values. Unknown keys are
                ignored; missing keys receive their model defaults.

        Returns:
            A new instance of the model populated from *input*.
        """
        ret = cls(**{k: input.get(k) for k in cls.get_keys()})

        return ret

    def json(self, shrink: bool = False) -> Dict[str, Any]:
        """Serialize this model to a plain dictionary via JSON round-trip.

        Args:
            shrink: If True, exclude fields whose value is None.

        Returns:
            A dictionary representation of the model.
        """
        return json.loads(self.model_dump_json(exclude_none=shrink))

    def __hash__(self) -> int:
        """Return a hash based on the non-None JSON representation."""
        return hash(self.model_dump_json(exclude_none=True))

    def __eq__(self, other: object) -> bool:
        """Check equality by comparing non-None JSON representations.

        Args:
            other: The object to compare against.

        Returns:
            True if both objects serialize to the same JSON string.
        """
        if not isinstance(other, DataModel):
            return NotImplemented

        return self.model_dump_json(exclude_none=True) == other.model_dump_json(
            exclude_none=True
        )

    def __repr__(self) -> str:
        """Return a developer-friendly string representation."""
        fields = ", ".join(
            f"{k}={v!r}" for k, v in self.model_dump(exclude_none=True).items()
        )
        return f"{self.__class__.__name__}({fields})"
