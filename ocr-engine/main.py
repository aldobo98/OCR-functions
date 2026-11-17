import json
import os
import requests
import numpy as np
import cv2
import easyocr
from dataclasses import dataclass, asdict
from typing import List
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from cloudevents.core.bindings.http import (
    from_http_event,
    HTTPMessage,
)

@dataclass
class ObjectDownloadRequest:
    url: str
    method: str
    fields: dict[str, list[str]]

@dataclass
class OcrRequest:
    jobId: str
    download: ObjectDownloadRequest
    callbackUrl: str

@dataclass
class OCRResult:
    bounding_box: List[List[int]]
    text: str
    confidence: float

@dataclass
class OCRResponse:
    results: List[OCRResult]

@dataclass
class OcrResponse:
    jobId: str
    step: str
    status: str
    data: OCRResponse
    error: str


def download_image(req: ObjectDownloadRequest):
    response = requests.request(
        method=req.method,
        url=req.url,
        data=req.fields or None,
        timeout=60,
    )
    response.raise_for_status()
    image_bytes = np.frombuffer(response.content, np.uint8)
    image = cv2.imdecode(image_bytes, cv2.IMREAD_COLOR)
    return image

def send_callback(data: OcrResponse, url: str):
    response = requests.post(
        url,
        json = asdict(data),
        timeout=10,
    )
    response.raise_for_status()

class OCRRequestHandler(BaseHTTPRequestHandler):

    def do_POST(self):
        try:
            # HTTP-kérés törzsének beolvasása
            content_length = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(content_length)

            # CloudEvent létrehozása a HTTP-kérésből
            event = from_http_event(
                HTTPMessage(
                    headers=dict(self.headers),
                    body=body,
                )
            )
            #Ha nem megfelelő a típus, akkor elutasítjuk
            if event.get_type() != "hu.ocr-backend.image.process":
                self.send_json(400, {"error": "Unsupported event type"})
                return
            #Kinyerjük az adatokat
            data = event.get_data()
            request = OcrRequest(
                jobId = data["jobId"],
                download= ObjectDownloadRequest(
                                url = data["download"]["url"],
                                method = data["download"]["method"],
                                fields = data["download"].get("fields", {}),
                            ),
                callbackUrl = data["callbackUrl"],
            )

            #Az image-t le kell tölteni
            image = download_image(request.download)
            # Szinkron OCR-feldolgozás
            reader = easyocr.Reader(['en'],download_enabled=True ,model_storage_directory="/models", gpu=False)
            rec_res = reader.readtext(image, paragraph=False, slope_ths=0.4, width_ths=0.7)
            #Összeállítjuk a választ a backend számára
            res_data = OCRResponse(
                results=[
                    OCRResult(
                        bounding_box=[
                            [int(coord) for coord in point]
                            for point in bbox
                        ],
                        text=text,
                        confidence=float(confidence)
                    )
                    for bbox, text, confidence in rec_res
                ]
            )
            results =   OcrResponse(
                jobId = request.jobId,
                step = "ocr_processing",
                status = "completed",
                data = res_data,
                error = ""
            )
            #Elküldjük a választ
            send_callback(results, request.callbackUrl)

        except Exception as e:
            print(f"Error processing event: {e}")


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8080"))

    server = ThreadingHTTPServer(("0.0.0.0", port), OCRRequestHandler)
    print(f"OCR service listening on port {port}")

    server.serve_forever()