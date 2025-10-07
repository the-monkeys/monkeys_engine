import concurrent.futures
import logging
import os
import sys

import grpc
import gw_report_pb2
import gw_report_pb2_grpc
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

import models

DB_HOST = os.getenv("POSTGRESQL_PRIMARY_DB_DB_HOST")
DB_PASSWORD = os.getenv("POSTGRESQL_PRIMARY_DB_DB_PASSWORD")
DB_USERNAME = os.getenv("POSTGRESQL_PRIMARY_DB_DB_USERNAME")
DB_NAME = os.getenv("POSTGRESQL_PRIMARY_DB_DB_NAME")
MICROSERVICE_REPORTS_SERVICE_INTERNAL_PORT = os.getenv("MICROSERVICE_REPORTS_SERVICE_INTERNAL_PORT")
MICROSERVICE_REPORTS_SERVICE_PORT = os.getenv("MICROSERVICE_REPORTS_SERVICE_PORT")
APP_ENV = os.getenv("APP_ENV", "development")

engine = create_engine(f"postgresql+psycopg2://{DB_USERNAME}:{DB_PASSWORD}@{DB_HOST}/{DB_NAME}", pool_size=20)

Session = sessionmaker(bind=engine)

if APP_ENV == "development":
    logging.getLogger("sqlalchemy.engine").setLevel(logging.INFO)

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO if APP_ENV == "development" else logging.WARN)
logger.addHandler(logging.StreamHandler(stream=sys.stdout))


class ReportsServiceServicer(gw_report_pb2_grpc.ReportsServiceServicer):
    def CreateReport(self, request: gw_report_pb2.CreateReportRequest, context):
        logger.info("Received request: ", request)

        report_body = {
            "reporter_id": request.reporter_id,
            "reason_type": request.reason_type,
            "reporter_notes": request.reporter_notes,
            "reported_type": request.reported_type
        }

        if request.reported_type == "BLOG":
            report_body["reported_blog_id"] = request.reported_id
        else:
            report_body["reported_user_id"] = request.reporter_id

        new_report = models.Report(**report_body)

        with Session() as session:
            session.add(new_report)
            session.commit()

    def FlagReport(self, request, context):
        pass


def serve():
    server = grpc.server(concurrent.futures.ThreadPoolExecutor(max_workers=20))
    server.add_insecure_port(f"[::]:{MICROSERVICE_REPORTS_SERVICE_INTERNAL_PORT}")
    gw_report_pb2_grpc.add_ReportsServiceServicer_to_server(ReportsServiceServicer(), server)

    server.start()
    server.wait_for_termination()

if __name__ == "__main__":
    logger.info(f"Starting reports service grpc server on {MICROSERVICE_REPORTS_SERVICE_PORT}")
    serve()
