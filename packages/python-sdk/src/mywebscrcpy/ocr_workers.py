"""Process-wide OCR admission: six unfinished tasks, with no waiting submitters."""
from concurrent.futures import ThreadPoolExecutor
import threading


class OCRBusyError(RuntimeError):
    """All OCR worker capacity is still owned by unfinished requests."""


class OCRWorkerPool:
    def __init__(self):
        self._slots = threading.BoundedSemaphore(6)
        self._executor = ThreadPoolExecutor(max_workers=6, thread_name_prefix="mywebscrcpy-ocr")

    def submit(self, function, *args, **kwargs):
        # Admission counts both queued and running work, so the executor's
        # internal queue cannot accumulate an unbounded number of submissions.
        if not self._slots.acquire(blocking=False):
            raise OCRBusyError("OCR workers busy: 6 unfinished tasks already admitted")
        try:
            future = self._executor.submit(function, *args, **kwargs)
        except BaseException:
            self._slots.release()
            raise
        future.add_done_callback(lambda _: self._slots.release())
        return future

    def shutdown(self):
        """Drain the pool when explicitly closing its owning runtime."""
        self._executor.shutdown(wait=True)


_pool = OCRWorkerPool()


def submit_ocr(function, *args, **kwargs):
    # Callers never cancel this future on timeout: an unfinished task retains
    # admission until it actually finishes, including work not yet scheduled.
    return _pool.submit(function, *args, **kwargs)
