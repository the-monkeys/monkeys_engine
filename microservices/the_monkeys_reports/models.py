from sqlmodel import Field, SQLModel


class Report(SQLModel, table=True):
    id: str = Field(primary_key=True)
    reason_type: str = Field()
    flag_count: int = Field(default=0)
    reporter_id: str = Field(foreign_key=True)
    reported_type: str = Field()
    reported_id: str = Field(foreign_key=True)
    reason: str | None = Field()
    status: str = Field(default="PENDING")
    reporter_notes: str | None = Field()
    moderator_id: str | None = Field()
    moderator_notes: str | None = Field()
