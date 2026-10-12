#!/usr/bin/env python3
"""Build the 问答通 smoke samples next to this script.

Each text-bearing file embeds one unique sentence. sample.png only needs
to finish parsing. reject.mp4 must be refused by the upload API.
"""

from __future__ import annotations

import json
import shutil
import struct
import subprocess
import zipfile
import zlib
from email.message import EmailMessage
from pathlib import Path

HERE = Path(__file__).resolve().parent

SENTENCES = {
    "sample.txt": "问答通样例原文 TXT-7f3a 只出现在这个文本文件里。",
    "sample.md": "问答通样例原文 MD-7f3b 只出现在这个 Markdown 文件里。",
    "sample.json": "问答通样例原文 JSON-7f3c 只出现在这个 JSON 文件里。",
    "sample.csv": "问答通样例原文 CSV-7f3d 只出现在这个 CSV 文件里。",
    "sample.html": "问答通样例原文 HTML-7f3e 只出现在这个 HTML 文件里。",
    "sample.mhtml": "问答通样例原文 MHTML-7f3f 只出现在这个 MHTML 文件里。",
    "sample.docx": "问答通样例原文 DOCX-7f40 只出现在这个 Word 文件里。",
    "sample.pptx": "问答通样例原文 PPTX-7f41 只出现在这个演示文稿里。",
    "sample.xlsx": "问答通样例原文 XLSX-7f42 只出现在这个表格文件里。",
    "sample.pdf": "问答通样例原文 PDF-7f43 只出现在这个 PDF 文件里。",
    "sample.epub": "问答通样例原文 EPUB-7f44 只出现在这本电子书里。",
    "sample.xmind": "问答通样例原文 XMIND-7f45 只出现在这张导图里。",
    "sample.doc": "问答通样例原文 DOC-7f46 只出现在这个旧版 Word 文件里。",
    "sample.ppt": "问答通样例原文 PPT-7f47 只出现在这个旧版演示文稿里。",
    "sample.xls": "问答通样例原文 XLS-7f48 只出现在这个旧版表格文件里。",
}


def _write(name: str, data: str | bytes) -> None:
    path = HERE / name
    if isinstance(data, str):
        path.write_text(data, encoding="utf-8")
    else:
        path.write_bytes(data)


def _png() -> bytes:
    def chunk(tag: bytes, payload: bytes) -> bytes:
        return (
            struct.pack(">I", len(payload))
            + tag
            + payload
            + struct.pack(">I", zlib.crc32(tag + payload) & 0xFFFFFFFF)
        )

    raw = b"\x00\xff\x00\x00"
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 1, 1, 8, 2, 0, 0, 0)) + chunk(
        b"IDAT", zlib.compress(raw)
    ) + chunk(b"IEND", b"")


def _pdf(sentence: str) -> bytes:
    # Helvetica WinAnsi cannot carry the Chinese sentence, so the text stream
    # uses a UTF-16 hex string that common extractors still keep as text.
    encoded = sentence.encode("utf-16-be").hex().upper()
    stream = f"BT /F1 12 Tf 36 720 Td <FEFF{encoded}> Tj ET\n".encode("ascii")
    objects = [
        b"1 0 obj<< /Type /Catalog /Pages 2 0 R >>endobj\n",
        b"2 0 obj<< /Type /Pages /Kids [3 0 R] /Count 1 >>endobj\n",
        b"3 0 obj<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "
        b"/Contents 4 0 R /Resources<< /Font<< /F1 5 0 R >> >> >>endobj\n",
        b"4 0 obj<< /Length %d >>stream\n" % len(stream) + stream + b"endstream\nendobj\n",
        b"5 0 obj<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>endobj\n",
    ]
    out = bytearray(b"%PDF-1.4\n")
    offsets = [0]
    for obj in objects:
        offsets.append(len(out))
        out.extend(obj)
    xref = len(out)
    out.extend(f"xref\n0 {len(offsets)}\n".encode())
    out.extend(b"0000000000 65535 f \n")
    for off in offsets[1:]:
        out.extend(f"{off:010d} 00000 n \n".encode())
    out.extend(
        f"trailer<< /Size {len(offsets)} /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF\n".encode()
    )
    return bytes(out)


def _zip(path: Path, items: list[tuple[str, str | bytes]], stored: set[str] | None = None) -> None:
    stored = stored or set()
    with zipfile.ZipFile(path, "w") as archive:
        for name, payload in items:
            data = payload.encode("utf-8") if isinstance(payload, str) else payload
            info = zipfile.ZipInfo(name)
            info.compress_type = zipfile.ZIP_STORED if name in stored else zipfile.ZIP_DEFLATED
            archive.writestr(info, data)


def _docx(sentence: str) -> None:
    document = f"""<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>{sentence}</w:t></w:r></w:p></w:body>
</w:document>
"""
    _zip(
        HERE / "sample.docx",
        [
            (
                "[Content_Types].xml",
                """<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>
""",
            ),
            (
                "_rels/.rels",
                """<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>
""",
            ),
            ("word/document.xml", document),
        ],
    )


def _pptx(sentence: str) -> None:
    slide = f"""<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree>
    <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
    <p:grpSpPr/>
    <p:sp>
      <p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
      <p:spPr/>
      <p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>{sentence}</a:t></a:r></a:p></p:txBody>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>
"""
    _zip(
        HERE / "sample.pptx",
        [
            (
                "[Content_Types].xml",
                """<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
</Types>
""",
            ),
            (
                "_rels/.rels",
                """<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>
""",
            ),
            (
                "ppt/presentation.xml",
                """<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst>
</p:presentation>
""",
            ),
            (
                "ppt/_rels/presentation.xml.rels",
                """<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>
""",
            ),
            ("ppt/slides/slide1.xml", slide),
        ],
    )


def _xlsx(sentence: str) -> None:
    sheet = f"""<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>{sentence}</t></is></c></row></sheetData>
</worksheet>
"""
    _zip(
        HERE / "sample.xlsx",
        [
            (
                "[Content_Types].xml",
                """<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>
""",
            ),
            (
                "_rels/.rels",
                """<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>
""",
            ),
            (
                "xl/workbook.xml",
                """<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>
</workbook>
""",
            ),
            (
                "xl/_rels/workbook.xml.rels",
                """<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>
""",
            ),
            ("xl/worksheets/sheet1.xml", sheet),
        ],
    )


def _epub(sentence: str) -> None:
    xhtml = f"""<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>sample</title></head>
<body><p>{sentence}</p></body></html>
"""
    _zip(
        HERE / "sample.epub",
        [
            ("mimetype", "application/epub+zip"),
            (
                "META-INF/container.xml",
                """<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
""",
            ),
            (
                "OEBPS/content.opf",
                """<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>sample</dc:title><dc:identifier id="uid">sample</dc:identifier><dc:language>zh</dc:language></metadata>
  <manifest><item id="c1" href="chapter.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="c1"/></spine>
</package>
""",
            ),
            ("OEBPS/chapter.xhtml", xhtml),
        ],
        stored={"mimetype"},
    )


def _xmind(sentence: str) -> None:
    content = [
        {
            "title": "问答通样例",
            "rootTopic": {"title": sentence, "children": {"attached": []}},
        }
    ]
    _zip(HERE / "sample.xmind", [("content.json", json.dumps(content, ensure_ascii=False))])


def _mhtml(sentence: str) -> None:
    root = EmailMessage()
    root["Subject"] = "问答通样例"
    root.make_related()
    page = EmailMessage()
    page.set_content(f"<html><body><p>{sentence}</p></body></html>", subtype="html")
    page["Content-Location"] = "https://example.invalid/sample"
    root.attach(page)
    _write("sample.mhtml", root.as_bytes())


def _legacy_from_textutil(src: Path, dest: Path, fmt: str) -> bool:
    try:
        subprocess.run(
            ["textutil", "-convert", fmt, "-output", str(dest), str(src)],
            check=True,
            capture_output=True,
        )
    except (OSError, subprocess.CalledProcessError):
        return False
    return dest.is_file() and dest.stat().st_size > 0


def _soffice_convert(src: Path, fmt: str, dest: Path) -> bool:
    soffice = shutil.which("soffice") or shutil.which("libreoffice")
    if not soffice:
        return False
    try:
        subprocess.run(
            [soffice, "--headless", "--convert-to", fmt, "--outdir", str(dest.parent), str(src)],
            check=True,
            capture_output=True,
            timeout=120,
        )
    except (OSError, subprocess.CalledProcessError, subprocess.TimeoutExpired):
        return False
    produced = dest.parent / f"{src.stem}.{fmt}"
    if produced != dest and produced.is_file():
        produced.replace(dest)
    return dest.is_file() and dest.stat().st_size > 0


def main() -> None:
    _write("sample.txt", SENTENCES["sample.txt"] + "\n")
    _write("sample.md", "# 样例\n\n" + SENTENCES["sample.md"] + "\n")
    _write(
        "sample.json",
        json.dumps({"title": "样例", "note": SENTENCES["sample.json"]}, ensure_ascii=False, indent=2) + "\n",
    )
    _write("sample.csv", "列\n" + SENTENCES["sample.csv"] + "\n")
    _write("sample.html", f"<html><body><p>{SENTENCES['sample.html']}</p></body></html>\n")
    _mhtml(SENTENCES["sample.mhtml"])
    _docx(SENTENCES["sample.docx"])
    _pptx(SENTENCES["sample.pptx"])
    _xlsx(SENTENCES["sample.xlsx"])
    _write("sample.pdf", _pdf(SENTENCES["sample.pdf"]))
    _epub(SENTENCES["sample.epub"])
    _xmind(SENTENCES["sample.xmind"])
    _write("sample.png", _png())
    _write("reject.mp4", b"\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom")

    plain = HERE / "_legacy.txt"
    plain.write_text(SENTENCES["sample.doc"] + "\n", encoding="utf-8")
    if not _legacy_from_textutil(plain, HERE / "sample.doc", "doc"):
        raise SystemExit("textutil could not write sample.doc")
    plain.unlink()
    if not (HERE / "sample.ppt").is_file():
        _soffice_convert(HERE / "sample.pptx", "ppt", HERE / "sample.ppt")
    if not (HERE / "sample.xls").is_file():
        _soffice_convert(HERE / "sample.xlsx", "xls", HERE / "sample.xls")

    manifest = []
    for name, sentence in SENTENCES.items():
        expect = "content"
        if not (HERE / name).is_file():
            expect = "missing"
        manifest.append({"file": name, "sentence": sentence, "expect": expect})
    manifest.append({"file": "sample.png", "sentence": "", "expect": "status"})
    manifest.append({"file": "reject.mp4", "sentence": "", "expect": "rejected"})
    missing = [item["file"] for item in manifest if item["expect"] == "missing"]
    # LibreOffice writes the old PowerPoint file when it is installed.
    # Excel is written by xlwt when that package is available, otherwise by LibreOffice.
    for name in ("sample.ppt", "sample.xls"):
        if not (HERE / name).is_file():
            manifest = [item for item in manifest if item["file"] != name]
    (HERE / "manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    if missing:
        print("legacy samples not generated:", ", ".join(missing))


if __name__ == "__main__":
    main()
