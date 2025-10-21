from sqlalchemy import String, INT, ForeignKey, CheckConstraint, MetaData, Table, create_engine
from sqlalchemy.dialects.mysql import BIGINT
from sqlalchemy.orm import DeclarativeBase, Mapped, mapped_column, relationship, sessionmaker

import os

DB = {
    "HOST": os.getenv("POSTGRESQL_PRIMARY_DB_DB_HOST"),
    "PASSWORD": os.getenv("POSTGRESQL_PRIMARY_DB_DB_PASSWORD"),
    "USERNAME": os.getenv("POSTGRESQL_PRIMARY_DB_DB_USERNAME"),
    "NAME": os.getenv("POSTGRESQL_PRIMARY_DB_DB_NAME")
}

engine = create_engine(f"postgresql+psycopg2://{DB["USERNAME"]}:{DB["PASSWORD"]}@{DB["HOST"]}/{DB["NAME"]}",
                       pool_size=20)

Session = sessionmaker(bind=engine)

metadata = MetaData()

blog = Table("blog", metadata, autoload_with=engine)
user_account = Table("user_account", metadata, autoload_with=engine)

class Base(DeclarativeBase):
    metadata = metadata
    pass


class Report(Base):
    __tablename__ = "reports"

    report_id: Mapped[int] = mapped_column(primary_key=True, autoincrement=True)
    reason_type: Mapped[str] = mapped_column(String)
    flag_count: Mapped[int] = mapped_column(INT, default=0)
    reporter_id: Mapped[BIGINT] = mapped_column(ForeignKey("user_account.id"))

    reported_type: Mapped[str] = mapped_column(String)

    reported_blog_id: Mapped[BIGINT] = mapped_column(ForeignKey("blog.id"), nullable=True)
    reported_user_id: Mapped[BIGINT] = mapped_column(ForeignKey("user_account.id"), nullable=True)

    status: Mapped[str] = mapped_column(String)
    reporter_notes: Mapped[str] = mapped_column(String)
    moderator_id: Mapped[BIGINT] = mapped_column(ForeignKey("user_account.id"))
    moderator_notes: Mapped[str] = mapped_column(String)

    verdict: Mapped[str] = mapped_column(String)

    __table_args__ = (
        CheckConstraint("status IN ('PENDING', 'IN_PROGRESS', 'RESOLVED')"),
        CheckConstraint("reported_type IN ('BLOG', 'USER')"),
        CheckConstraint("reason_type IN ('SPAM', 'ABUSE', 'NSFW', 'MISINFORMATION', 'OTHER')"),
        CheckConstraint("(reported_blog_id) IS NOT NULL) + (reported_user_id IS NOT NULL) = 1", name="exactly_one_fk")
    )


# Association table for user and reports
class UserReportFlag(Base):
    __tablename__ = "user_reports_flags"

    user_id: Mapped[BIGINT] = mapped_column(ForeignKey("user_account.id"), primary_key=True)
    report_id: Mapped[str] = mapped_column(ForeignKey("reports.report_id"), primary_key=True)
