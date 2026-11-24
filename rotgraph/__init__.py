"""rotgraph — orchestrator for rotating third-party credentials.

The package is laid out as a small pipeline:

``plan``      read a batch description and decide in which order its keys run
``journal``   append-only record of what the orchestrator intends to do
``providers`` one adapter per external vendor
``notify``    push a freshly minted key to internal consumers and collect acks
``probe``     ask a vendor whether a key is usable right now
``runner``    drive one batch from start to finish
``http``      the small admin surface used by ``rotgraphd``
"""

__version__ = "0.4.0"
