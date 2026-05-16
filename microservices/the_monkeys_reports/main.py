import concurrent.futures
import logging
import os
import sys

import grpc
from sqlalchemy.exc import IntegrityError, SQLAlchemyError

import gw_report_pb2
import gw_report_pb2_grpc

import models
from gw_report_pb2 import CreateReportResponse
from models import Session, DB

MICROSERVICE_REPORTS_SERVICE_INTERNAL_PORT = os.getenv("MICROSERVICE_REPORTS_SERVICE_INTERNAL_PORT")
MICROSERVICE_REPORTS_SERVICE_PORT = os.getenv("MICROSERVICE_REPORTS_SERVICE_PORT")
APP_ENV = os.getenv("APP_ENV", "development")


if APP_ENV == "development":
    logging.getLogger("sqlalchemy.engine").setLevel(logging.INFO)

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO if APP_ENV == "development" else logging.WARN)

handler = logging.StreamHandler(stream=sys.stdout)
formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')

handler.setFormatter(formatter)

logger.addHandler(handler)


class ReportsServiceServicer(gw_report_pb2_grpc.ReportsServiceServicer):
    def CreateReport(self, request: gw_report_pb2.CreateReportRequest, context) -> CreateReportResponse | None:
        logger.info(f"Received request\n{request}")

        report_body = {
            "reporter_id": int(request.reporter_id),
            "reason_type": request.reason_type,
            "reporter_notes": request.reporter_notes,
            "reported_type": request.reported_type
        }

        if request.reported_type == "BLOG":
            report_body["reported_blog_id"] = int(request.reported_id)
        else:
            report_body["reported_user_id"] = int(request.reported_id)

        new_report = models.Report(**report_body)

        try:
            with Session() as session:
                session.add(new_report)
                session.commit()
        except IntegrityError as e:
            logger.error(e)
            return gw_report_pb2.CreateReportResponse(status=0, message="", error=e._message())
        except SQLAlchemyError as e:
            logger.error(e)
            return gw_report_pb2.CreateReportResponse(status=0, message="", error=e._message())
        else:
            logger.info("Response sent")
            return gw_report_pb2.CreateReportResponse(status=1, message="Created report successfully", error="")

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
    logger.info(f"Connecting to DB with credentials {DB}")
    serve()
