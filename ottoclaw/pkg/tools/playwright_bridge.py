#!/usr/bin/env python3
"""
Playwright Bridge for ottoclaw desktop_browser tool.
Input:  JSON on stdin  { "steps": [...], "session_file": "...", "headless": false }
Output: JSON on stdout { "success": true, "results": [...] }

Each step:
  { "action": "navigate",     "url": "https://..." }
  { "action": "click",        "selector": "CSS" }
  { "action": "click",        "x": 100, "y": 200 }
  { "action": "type_text",    "selector": "CSS", "text": "..." }
  { "action": "type_text",    "text": "..." }          # types at current focus
  { "action": "screenshot",   "path": "/tmp/ss.png" }
  { "action": "get_text",     "selector": "CSS" }
  { "action": "get_url" }
  { "action": "wait_for",     "selector": "CSS", "timeout_ms": 10000 }
  { "action": "evaluate_js",  "js": "document.title" }
  { "action": "scroll",       "delta_y": 500 }
  { "action": "press_key",    "key": "Enter" }
  { "action": "select_option","selector": "CSS", "value": "..." }
  { "action": "hover",        "selector": "CSS" }
"""
import sys
import json
import os
import traceback


def run_steps(params):
    from playwright.sync_api import sync_playwright

    steps = params.get("steps", [])
    # Support single-action call: {"action": "...", ...}
    if not steps and "action" in params:
        steps = [params]

    session_file = params.get(
        "session_file",
        os.path.expanduser("~/.ottoclaw/browser_session.json")
    )
    os.makedirs(os.path.dirname(session_file), exist_ok=True)

    # Use headed when DISPLAY is available, unless caller forces headless
    has_display = os.environ.get("DISPLAY", "").strip() != ""
    headless = params.get("headless", not has_display)

    results = []

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=headless)

        ctx_args = {}
        if os.path.exists(session_file):
            ctx_args["storage_state"] = session_file

        context = browser.new_context(
            viewport={"width": 1280, "height": 800},
            **ctx_args
        )
        page = context.new_page()

        for step in steps:
            action = step.get("action", "")
            step_result = {"action": action}

            try:
                if action == "navigate":
                    url = step["url"]
                    page.goto(url, wait_until="domcontentloaded", timeout=30000)
                    step_result.update({"url": page.url, "title": page.title()})

                elif action == "click":
                    selector = step.get("selector")
                    x, y = step.get("x"), step.get("y")
                    if selector:
                        page.click(selector, timeout=10000)
                    elif x is not None and y is not None:
                        page.mouse.click(float(x), float(y))
                    else:
                        raise ValueError("click requires 'selector' or 'x'+'y'")
                    step_result["url"] = page.url

                elif action == "type_text":
                    text = step.get("text", "")
                    selector = step.get("selector")
                    if selector:
                        page.fill(selector, text)
                    else:
                        page.keyboard.type(text)
                    step_result["chars"] = len(text)

                elif action == "screenshot":
                    path = step.get("path", "/tmp/ottoclaw_screenshot.png")
                    full_page = step.get("full_page", False)
                    page.screenshot(path=path, full_page=full_page)
                    step_result["path"] = path

                elif action == "get_text":
                    selector = step.get("selector", "body")
                    text = page.text_content(selector, timeout=10000) or ""
                    max_len = step.get("max_len", 3000)
                    step_result["text"] = text[:max_len]

                elif action == "get_url":
                    step_result.update({"url": page.url, "title": page.title()})

                elif action == "wait_for":
                    selector = step["selector"]
                    timeout = step.get("timeout_ms", 10000)
                    page.wait_for_selector(selector, timeout=timeout)
                    step_result["found"] = True

                elif action == "evaluate_js":
                    js = step["js"]
                    value = page.evaluate(js)
                    step_result["result"] = str(value)[:2000]

                elif action == "scroll":
                    delta_y = step.get("delta_y", 500)
                    delta_x = step.get("delta_x", 0)
                    page.mouse.wheel(delta_x, delta_y)
                    step_result["scrolled"] = True

                elif action == "press_key":
                    key = step["key"]
                    page.keyboard.press(key)
                    step_result["key"] = key

                elif action == "select_option":
                    selector = step["selector"]
                    value = step["value"]
                    page.select_option(selector, value)
                    step_result["selected"] = value

                elif action == "hover":
                    selector = step["selector"]
                    page.hover(selector, timeout=10000)
                    step_result["hovered"] = True

                else:
                    step_result["error"] = f"unknown action: {action}"

                step_result["ok"] = True

            except Exception as e:
                step_result["ok"] = False
                step_result["error"] = str(e)

            results.append(step_result)

            # Stop on error unless caller wants to continue
            if not step_result.get("ok") and not params.get("continue_on_error"):
                break

        # Persist session (cookies, localStorage)
        try:
            context.storage_state(path=session_file)
        except Exception:
            pass

        context.close()
        browser.close()

    return results


if __name__ == "__main__":
    try:
        params = json.load(sys.stdin)
        results = run_steps(params)
        all_ok = all(r.get("ok", False) for r in results)
        print(json.dumps({"success": all_ok, "results": results}))
    except Exception as e:
        print(json.dumps({
            "success": False,
            "error": str(e),
            "traceback": traceback.format_exc()
        }))
