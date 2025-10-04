# Put this configuration into ~/.config/lagrange/mimehooks.txt to enable:
#
# Echomail header styling
# text/gemini
# /usr/bin/python3;/INSTALL_PATH/echomail.py
#
# where INSTALL_PATH is where the echomail.py file actually is - I use ~/.config/lagrange/scripts

# Width of the header frame for echomail messages.
WIDTH = 60

# Whether to use markdown-like emphasis styling.
# Enabling this assumes that "Font Style" ANSI escapes are
# enabled in Preferences.
EMPHASIS = True

import re
import sys
from datetime import datetime
from zoneinfo import ZoneInfo

doc = sys.stdin.read()

pt_em = re.compile(r"_([^_]+)_")
pt_strong = re.compile(r"\*\*([^*]+)\*\*")

output_lines = []
preformatted = False
echomail = False
echomail_header = []


def process_header():
    """
    Process accumulated header lines into new output.
    """
    # First, turn them into a dict.
    fields = {}
    for header_line in echomail_header:
        chunks = header_line.split(": ", 1)
        fields[chunks[0]] = chunks[1].strip()

    # Convert date to local timezone.
    date_string = fields.get("Date")
    if date_string:
        dt = (datetime.strptime(date_string, "%Y-%m-%d %H:%M:%S")).replace(
            tzinfo=ZoneInfo("UTC")
        )
        dt_local = dt.astimezone().strftime("%Y-%m-%d %H:%M:%S")
        fields["Date"] = dt_local

    # Now produce a completely new decorative header.
    output_lines.append("```Message header")
    output_lines.append("┌" + "─" * (WIDTH - 1))

    from_string = fields.get("From")
    if not from_string:
        from_string = "Anonymous user at " + fields["Sender"]
    output_lines.append("│ From: " + from_string)

    to_string = fields.get("To")
    if not to_string:
        to_string = "All"
    output_lines.append("│ To:   " + to_string)

    date = fields.get("Date")
    if date:
        output_lines.append("│ Date: " + date)

    output_lines.append("└" + "─" * (WIDTH - 1))
    output_lines.append("```")

    subj = fields.get("Subj")
    if subj:
        output_lines.append("# " + subj)


for line in doc.split("\n"):
    # This branch captures the echomail header and then
    # spews out the processed version.
    if line.startswith("```"):
        # if it's an echomail header block, we swallow the line,
        # we're going to replace the entire block.
        preformatted = not preformatted
        if line.startswith("```Echomail"):
            echomail = True
            continue
        if not preformatted:
            if echomail:
                # If we closed up an echomail header block,
                # present our findings.
                process_header()
                echomail = False
                continue
    if not line.startswith("```") and echomail:
        echomail_header.append(line)
        continue

    # This branch converts _ into ansi escapes if EMPHASIS is enabled.
    if EMPHASIS and not line.startswith("=>") and not preformatted:
        line = pt_em.subn("\x1b[3m\\1\x1b[0m", line)[0]
        line = pt_strong.subn("\x1b[1m\\1\x1b[0m", line)[0]
    output_lines.append(line)

print("20 text/gemini\r")
print("\n".join(output_lines))
