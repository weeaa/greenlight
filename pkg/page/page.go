package page

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

func init() {
	rand.NewSource(time.Now().UnixNano())
}

type BrowserInterface interface {
	SendCommandWithoutResponse(method string, params map[string]interface{}) error
	SendCommandWithResponse(method string, params map[string]interface{}) (map[string]interface{}, error)
}

type Page struct {
	browser BrowserInterface
}

type Locator struct {
	page     *Page
	selector string
}

func NewPage(browser BrowserInterface) *Page {
	return &Page{browser: browser}
}

func (p *Page) Locator(selector string) *Locator {
	return &Locator{
		page:     p,
		selector: selector,
	}
}

func (p *Page) YellowLight(milliseconds int) {
	time.Sleep(time.Duration(milliseconds) * time.Millisecond)
}

func (p *Page) Goto(url string) error {
	if err := p.browser.SendCommandWithoutResponse("Page.enable", nil); err != nil {
		return fmt.Errorf("failed to enable page domain: %w", err)
	}
	if err := p.browser.SendCommandWithoutResponse("Network.enable", nil); err != nil {
		return fmt.Errorf("failed to enable network domain: %w", err)
	}

	params := map[string]interface{}{
		"url": url,
	}
	if err := p.browser.SendCommandWithoutResponse("Page.navigate", params); err != nil {
		return fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	return nil
}

func (l *Locator) elementExists() (bool, error) {
	params := map[string]interface{}{
		"expression":    fmt.Sprintf(`document.querySelector("%s") !== null`, l.selector),
		"returnByValue": true,
	}
	response, err := l.page.browser.SendCommandWithResponse("Runtime.evaluate", params)
	if err != nil {
		return false, err
	}

	if result, ok := response["result"].(map[string]interface{}); ok {
		if nestedResult, ok := result["result"].(map[string]interface{}); ok {
			if value, ok := nestedResult["value"].(bool); ok {
				return value, nil
			}
		}
	}
	return false, fmt.Errorf("unexpected response format: %v", response)
}

func (l *Locator) Fill(value string) error {
	timeout := 30 * time.Second
	interval := 350 * time.Millisecond
	startTime := time.Now()

	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout exceeded while waiting for selector: %s", l.selector)
		}

		exists, err := l.elementExists()
		if err != nil {
			time.Sleep(interval)
			continue
		}

		if exists {
			// Focus element
			if err = l.page.browser.SendCommandWithoutResponse("Runtime.evaluate", map[string]interface{}{
				"expression": fmt.Sprintf(`document.querySelector("%s").focus()`, l.selector),
			}); err != nil {
				return err
			}

			if err = l.page.browser.SendCommandWithoutResponse("Input.dispatchKeyEvent", map[string]interface{}{
				"type":      "keyDown",
				"modifiers": 2,
				"key":       "a",
			}); err != nil {
				return err
			}

			if err = l.page.browser.SendCommandWithoutResponse("Input.dispatchKeyEvent", map[string]interface{}{
				"type": "keyDown",
				"key":  "Backspace",
			}); err != nil {
				return err
			}

			return l.page.browser.SendCommandWithoutResponse("Input.insertText", map[string]interface{}{
				"text": value,
			})
		}

		time.Sleep(interval)
	}
}

func (l *Locator) Click() error {
	timeout := 30 * time.Second
	interval := 350 * time.Millisecond
	startTime := time.Now()

	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout exceeded while waiting for selector: %s", l.selector)
		}

		exists, err := l.elementExists()
		if err != nil {
			time.Sleep(interval)
			continue
		}

		if exists {
			params := map[string]interface{}{
				"expression":   fmt.Sprintf(`document.querySelector("%s").click()`, l.selector),
				"awaitPromise": true,
			}

			if err = l.page.browser.SendCommandWithoutResponse("Runtime.evaluate", params); err != nil {
				return fmt.Errorf("failed to click on selector %s: %w", l.selector, err)
			}

			return nil
		}

		time.Sleep(interval)
	}
}

func (l *Locator) TypeSequentially(text string, delayMs int) {
	timeout := 30 * time.Second
	interval := 350 * time.Millisecond
	startTime := time.Now()

	for {
		if time.Since(startTime) > timeout {
			log.Fatalf("Timeout exceeded while waiting for selector: %s", l.selector)
		}

		exists, err := l.elementExists()
		if err != nil {
			time.Sleep(interval)
			continue
		}

		if exists {
			l.page.browser.SendCommandWithoutResponse("Runtime.evaluate", map[string]interface{}{
				"expression": fmt.Sprintf(`document.querySelector("%s").focus()`, l.selector),
			})

			for _, char := range text {
				l.page.browser.SendCommandWithoutResponse("Input.insertText", map[string]interface{}{
					"text": string(char),
				})
				time.Sleep(time.Duration(delayMs) * time.Millisecond)
			}

			return
		}
		time.Sleep(interval)
	}
}

func (l *Locator) InnerText() (string, error) {
	timeout := 30 * time.Second
	interval := 350 * time.Millisecond
	startTime := time.Now()

	for {
		if time.Since(startTime) > timeout {
			return "", fmt.Errorf("timeout exceeded while waiting for selector: %s", l.selector)
		}

		exists, err := l.elementExists()
		if err != nil {
			time.Sleep(interval)
			continue
		}

		if exists {
			params := map[string]interface{}{
				"expression":    fmt.Sprintf(`document.querySelector("%s").innerText`, l.selector),
				"returnByValue": true,
			}

			response, err := l.page.browser.SendCommandWithResponse("Runtime.evaluate", params)
			if err != nil {
				return "", fmt.Errorf("failed to get inner text for selector %s: %w", l.selector, err)
			}

			if result, ok := response["result"].(map[string]interface{}); ok {
				if value, ok := result["value"].(string); ok {
					return value, nil
				}
			}

			return "", fmt.Errorf("unexpected response format for inner text: %v", response)
		}

		time.Sleep(interval)
	}
}

func (l *Locator) TypeWithMistakes(text string, delayMs int) error {
	timeout := 30 * time.Second
	interval := 350 * time.Millisecond
	startTime := time.Now()

	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("timeout exceeded while waiting for selector: %s", l.selector)
		}

		exists, err := l.elementExists()
		if err != nil {
			time.Sleep(interval)
			continue
		}

		if exists {
			if err = l.page.browser.SendCommandWithoutResponse("Runtime.evaluate", map[string]interface{}{
				"expression": fmt.Sprintf(`document.querySelector("%s").focus()`, l.selector),
			}); err != nil {
				return err
			}

			for _, char := range text {
				if rand.Float32() < 0.4 {
					wrongChar := string(rand.Int31n(26) + 'a')

					if err = l.page.browser.SendCommandWithoutResponse("Input.insertText", map[string]interface{}{
						"text": wrongChar,
					}); err != nil {
						return err
					}
					time.Sleep(time.Duration(delayMs) * time.Millisecond)

					if err = l.page.browser.SendCommandWithoutResponse("Input.dispatchKeyEvent", map[string]interface{}{
						"type":                  "rawKeyDown",
						"key":                   "Backspace",
						"windowsVirtualKeyCode": 8,
						"nativeVirtualKeyCode":  8,
					}); err != nil {
						return err
					}

					time.Sleep(time.Duration(delayMs) * time.Millisecond)
				}

				if err = l.page.browser.SendCommandWithoutResponse("Input.insertText", map[string]interface{}{
					"text": string(char),
				}); err != nil {
					return err
				}

				time.Sleep(time.Duration(delayMs) * time.Millisecond)
			}

			return nil
		}

		time.Sleep(interval)
	}
}

func (p *Page) GetHtml() (string, error) {
	params := map[string]interface{}{
		"expression":    "document.documentElement.outerHTML",
		"returnByValue": true,
	}

	response, err := p.browser.SendCommandWithResponse("Runtime.evaluate", params)
	if err != nil {
		return "", err
	}

	if resultMap, ok := response["result"].(map[string]interface{}); ok {
		if nestedResult, ok := resultMap["result"].(map[string]interface{}); ok {
			if htmlContent, ok := nestedResult["value"].(string); ok {
				return htmlContent, nil
			}
		}
	}

	return "", fmt.Errorf("unexpected response format: %v", response)
}

func (p *Page) Refresh() error {
	if err := p.browser.SendCommandWithoutResponse("Page.reload", nil); err != nil {
		return fmt.Errorf("failed to refresh the page: %w", err)
	}
	return nil
}

func (p *Page) GetHttpStatus() (int, error) {
	params := map[string]interface{}{
		"expression": `(() => {
			const navEntries = window.performance.getEntriesByType('navigation');
			if (navEntries.length === 0) return 0;
			return navEntries[0].responseStatus;
		})()`,
		"returnByValue": true,
	}

	response, err := p.browser.SendCommandWithResponse("Runtime.evaluate", params)
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate JS for status: %w", err)
	}

	if result, ok := response["result"].(map[string]interface{}); ok {
		if nestedResult, ok := result["result"].(map[string]interface{}); ok {
			if status, ok := nestedResult["value"].(float64); ok {
				return int(status), nil
			}
		}
	}

	return 0, fmt.Errorf("unexpected response format: %v", response)
}
