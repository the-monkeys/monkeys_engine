from fastapi import FastAPI, Request

from models import Report

app = FastAPI()

@app.get("/")
async def root():
    return {"hello": "world"}


@app.post("/")
async def create_report(request: Request):
    new_report = Report()

    pass
