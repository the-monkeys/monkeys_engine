from sqlalchemy import String, INT, ForeignKey, CheckConstraint
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column


class Base(DeclarativeBase):
    pass


class Report(Base):
    __tablename__ = "reports"

    id: Mapped[str] = mapped_column(primary_key=True)
    reason_type: Mapped[str] = mapped_column(String)
    flag_count: Mapped[int] = mapped_column(INT, default=0)
    reporter_id: Mapped[int] = mapped_column(ForeignKey("user_account.id"))

    reported_type: Mapped[str] = mapped_column(String)

    reported_blog_id: Mapped[str] = mapped_column(ForeignKey("blog.id"), nullable=True)
    reported_user_id: Mapped[int] = mapped_column(ForeignKey("user_account.id"), nullable=True)

    reason: Mapped[str] = mapped_column(String)
    status: Mapped[str] = mapped_column(String)
    reporter_notes: Mapped[str] = mapped_column(String)
    moderator_id: Mapped[int] = mapped_column(ForeignKey("user_account.id"))
    moderator_notes: Mapped[str] = mapped_column(String)

    __table_args__ = (
        CheckConstraint("status IN ('PENDING', 'IN_PROGRESS', 'RESOLVED')"),
        CheckConstraint("reported_type IN ('BLOG', 'USER', 'COMMENT')"),
        CheckConstraint("reason_type IN ('SPAM_OR_COPYRIGHT', 'HARASSMENT', 'OTHER')"),
        CheckConstraint("(reported_blog_id) IS NOT NULL) + (reported_user_id IS NOT NULL) = 1", name="exactly_one_fk")
    )


# Association table for user and reports
class UserReportFlag(Base):
    __tablename__ = "user_reports_flags"

    user_id: Mapped[str] = mapped_column(ForeignKey("user_account.id"), primary_key=True)
    report_id: Mapped[str] = mapped_column(ForeignKey("reports.id"), primary_key=True)
