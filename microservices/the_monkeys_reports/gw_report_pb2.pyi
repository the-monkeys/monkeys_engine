from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class Report(_message.Message):
    __slots__ = ("report_id", "reason_type", "flag_count", "reporter_id", "reported_type", "reported_id", "status", "reporter_notes", "moderator_id", "moderator_notes")
    REPORT_ID_FIELD_NUMBER: _ClassVar[int]
    REASON_TYPE_FIELD_NUMBER: _ClassVar[int]
    FLAG_COUNT_FIELD_NUMBER: _ClassVar[int]
    REPORTER_ID_FIELD_NUMBER: _ClassVar[int]
    REPORTED_TYPE_FIELD_NUMBER: _ClassVar[int]
    REPORTED_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    REPORTER_NOTES_FIELD_NUMBER: _ClassVar[int]
    MODERATOR_ID_FIELD_NUMBER: _ClassVar[int]
    MODERATOR_NOTES_FIELD_NUMBER: _ClassVar[int]
    report_id: str
    reason_type: str
    flag_count: int
    reporter_id: str
    reported_type: str
    reported_id: str
    status: str
    reporter_notes: str
    moderator_id: str
    moderator_notes: str
    def __init__(self, report_id: _Optional[str] = ..., reason_type: _Optional[str] = ..., flag_count: _Optional[int] = ..., reporter_id: _Optional[str] = ..., reported_type: _Optional[str] = ..., reported_id: _Optional[str] = ..., status: _Optional[str] = ..., reporter_notes: _Optional[str] = ..., moderator_id: _Optional[str] = ..., moderator_notes: _Optional[str] = ...) -> None: ...

class CreateReportRequest(_message.Message):
    __slots__ = ("reason_type", "reporter_id", "reported_type", "reported_id", "reporter_notes")
    REASON_TYPE_FIELD_NUMBER: _ClassVar[int]
    REPORTER_ID_FIELD_NUMBER: _ClassVar[int]
    REPORTED_TYPE_FIELD_NUMBER: _ClassVar[int]
    REPORTED_ID_FIELD_NUMBER: _ClassVar[int]
    REPORTER_NOTES_FIELD_NUMBER: _ClassVar[int]
    reason_type: str
    reporter_id: str
    reported_type: str
    reported_id: str
    reporter_notes: str
    def __init__(self, reason_type: _Optional[str] = ..., reporter_id: _Optional[str] = ..., reported_type: _Optional[str] = ..., reported_id: _Optional[str] = ..., reporter_notes: _Optional[str] = ...) -> None: ...

class CreateReportResponse(_message.Message):
    __slots__ = ("status", "message", "error")
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    status: int
    message: str
    error: str
    def __init__(self, status: _Optional[int] = ..., message: _Optional[str] = ..., error: _Optional[str] = ...) -> None: ...

class FlagReportRequest(_message.Message):
    __slots__ = ("report_id", "user_id")
    REPORT_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    report_id: str
    user_id: str
    def __init__(self, report_id: _Optional[str] = ..., user_id: _Optional[str] = ...) -> None: ...

class FlagReportResponse(_message.Message):
    __slots__ = ("status", "message", "error")
    STATUS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    status: int
    message: str
    error: str
    def __init__(self, status: _Optional[int] = ..., message: _Optional[str] = ..., error: _Optional[str] = ...) -> None: ...
