https://codexplorer.medium.com/5-design-patterns-that-transformed-my-go-code-from-chaos-to-clean-df397ac79c23

5 Design Patterns That Transformed My Go Code from Chaos to Clean Codexplorer
Codexplorer

Follow 12 min read · Sep 27, 2025 36

1

Let me tell you about the refactoring project that taught me why design patterns
matter.

I inherited a Go codebase that was… well, let’s call it “organically grown.”
There was a single 800-line function that handled user notifications. It had
nested if statements for different notification types, hardcoded email
templates, and every time someone wanted to add a new notification method, they
just added another if statement to the pile.

Adding SMS notifications required touching 12 different files. The code was
fragile, hard to test, and impossible to extend without breaking something else.
That’s when I discovered that design patterns aren’t just academic concepts —
they’re practical tools for organizing code that actually needs to work.

Here are the 5 patterns that transformed that chaotic codebase into something
maintainable, and how you can use them in your Go projects.

Press enter or click to view image in full size

golang design patterns Pattern 1: Strategy — Stop the if/else Madness The
Problem: You have different ways to do the same thing, and your code is full of
switch statements or if/else chains.

I see this pattern everywhere in Go codebases:

// The nightmare that keeps growing func processPayment(amount float64, method
string, details map[string]string) error { if method == "credit_card" { // 50
lines of credit card logic cardNumber := details["card_number"] if cardNumber ==
"" { return errors.New("card number required") } // ... more credit card stuff

    } else if method == "paypal" {
        // 40 lines of PayPal logic
        email := details["email"]
        if email == "" {
            return errors.New("email required")
        }
        // ... more PayPal stuff

    } else if method == "bank_transfer" {
        // 60 lines of bank transfer logic
        accountNumber := details["account"]
        if accountNumber == "" {
            return errors.New("account number required")
        }
        // ... more bank transfer stuff

    } else if method == "crypto" {
        // Someone just added this last week...
        // ... 70 more lines
    } else {
        return errors.New("unsupported payment method")
    }

    return nil

} Every time you add a new payment method, this function gets bigger and more
fragile.

The Strategy Pattern Solution:

package main

import ( "errors" "fmt" )

// Strategy interface - all payment methods implement this type PaymentStrategy
interface { Process(amount float64, details map[string]string) error
Validate(details map[string]string) error }

// Concrete strategies type CreditCardPayment struct{}

func (cc \*CreditCardPayment) Validate(details map[string]string) error { if
details["card_number"] == "" { return errors.New("card number required") } if
details["cvv"] == "" { return errors.New("CVV required") } return nil }

func (cc \*CreditCardPayment) Process(amount float64, details map[string]string)
error { if err := cc.Validate(details); err != nil { return err }

    // Credit card processing logic
    fmt.Printf("Processing $%.2f via credit card ending in %s\n",
        amount, details["card_number"][len(details["card_number"])-4:])

    return nil

}

type PayPalPayment struct{}

func (pp \*PayPalPayment) Validate(details map[string]string) error { if
details["email"] == "" { return errors.New("email required") } return nil }

func (pp \*PayPalPayment) Process(amount float64, details map[string]string)
error { if err := pp.Validate(details); err != nil { return err }

    // PayPal processing logic
    fmt.Printf("Processing $%.2f via PayPal for %s\n", amount, details["email"])

    return nil

}

type BankTransferPayment struct{}

func (bt \*BankTransferPayment) Validate(details map[string]string) error { if
details["account_number"] == "" { return errors.New("account number required") }
if details["routing_number"] == "" { return errors.New("routing number
required") } return nil }

func (bt \*BankTransferPayment) Process(amount float64, details
map[string]string) error { if err := bt.Validate(details); err != nil { return
err }

    // Bank transfer processing logic
    fmt.Printf("Processing $%.2f via bank transfer to account %s\n",
        amount, details["account_number"])

    return nil

}

// Context that uses the strategy type PaymentProcessor struct { strategies
map[string]PaymentStrategy }

func NewPaymentProcessor() \*PaymentProcessor { return &PaymentProcessor{
strategies: map[string]PaymentStrategy{ "credit_card": &CreditCardPayment{},
"paypal": &PayPalPayment{}, "bank_transfer": &BankTransferPayment{}, }, } }

func (pp \*PaymentProcessor) RegisterStrategy(name string, strategy
PaymentStrategy) { pp.strategies[name] = strategy }

func (pp \*PaymentProcessor) Process(amount float64, method string, details
map[string]string) error { strategy, exists := pp.strategies[method] if !exists
{ return fmt.Errorf("unsupported payment method: %s", method) }

    return strategy.Process(amount, details)

}

// Adding a new payment method is now trivial type CryptoPayment struct{}

func (cp \*CryptoPayment) Validate(details map[string]string) error { if
details["wallet_address"] == "" { return errors.New("wallet address required") }
return nil }

func (cp \*CryptoPayment) Process(amount float64, details map[string]string)
error { if err := cp.Validate(details); err != nil { return err }

    fmt.Printf("Processing $%.2f via crypto to %s\n", amount, details["wallet_address"])
    return nil

}

func main() { processor := NewPaymentProcessor()

    // Add new payment method without touching existing code
    processor.RegisterStrategy("crypto", &CryptoPayment{})

    // Use the processor
    err := processor.Process(100.0, "credit_card", map[string]string{
        "card_number": "1234567890123456",
        "cvv":        "123",
    })

    if err != nil {
        fmt.Printf("Payment failed: %v\n", err)
    }

} Now adding a new payment method means implementing the interface and
registering it. No touching existing code, no giant if statements.

Pattern 2: Observer — Decouple Event Handling The Problem: When something
happens in your system, multiple other parts need to know about it, but you
don’t want tight coupling.

The wrong way looks like this:

// Tightly coupled nightmare func processOrder(order \*Order) error { if err :=
order.Save(); err != nil { return err }

    // Oh no, now we need to do 5 different things...
    sendConfirmationEmail(order)
    updateInventory(order)
    createShippingLabel(order)
    notifyWarehouse(order)
    updateAnalytics(order)

    // Next week: "Can we also send an SMS?"
    // The week after: "Can we post to Slack?"

    return nil

} The Observer Pattern Solution:

package main

import ( "fmt" "time" )

// Event interface type Event interface { Type() string Data() interface{} }

// Observer interface type Observer interface { Handle(event Event) error }

// Event dispatcher type EventDispatcher struct { observers
map[string][]Observer }

func NewEventDispatcher() \*EventDispatcher { return &EventDispatcher{
observers: make(map[string][]Observer), } }

func (ed \*EventDispatcher) Subscribe(eventType string, observer Observer) {
ed.observers[eventType] = append(ed.observers[eventType], observer) }

func (ed \*EventDispatcher) Dispatch(event Event) { if observers, exists :=
ed.observers[event.Type()]; exists { for \_, observer := range observers { // In
a real system, you might want to handle errors // or run these in goroutines go
observer.Handle(event) } } }

// Concrete event type OrderPlacedEvent struct { order \*Order timestamp
time.Time }

func (ope \*OrderPlacedEvent) Type() string { return "order.placed" }

func (ope \*OrderPlacedEvent) Data() interface{} { return ope.order }

// Concrete observers type EmailNotifier struct{}

func (en *EmailNotifier) Handle(event Event) error { if event.Type() ==
"order.placed" { order := event.Data().(*Order) fmt.Printf("📧 Sending
confirmation email for order %s to %s\n", order.ID, order.CustomerEmail) }
return nil }

type InventoryUpdater struct{}

func (iu *InventoryUpdater) Handle(event Event) error { if event.Type() ==
"order.placed" { order := event.Data().(*Order) fmt.Printf("📦 Updating
inventory for %d items\n", len(order.Items)) } return nil }

type ShippingLabelCreator struct{}

func (slc *ShippingLabelCreator) Handle(event Event) error { if event.Type() ==
"order.placed" { order := event.Data().(*Order) fmt.Printf("🏷️ Creating shipping
label for order %s\n", order.ID) } return nil }

type SlackNotifier struct{}

func (sn *SlackNotifier) Handle(event Event) error { if event.Type() ==
"order.placed" { order := event.Data().(*Order) fmt.Printf("💬 Posting to Slack:
New order %s for $%.2f\n", order.ID, order.Total) } return nil }

// Your business logic stays clean type OrderService struct { dispatcher
\*EventDispatcher }

func NewOrderService(dispatcher *EventDispatcher) *OrderService { return
&OrderService{dispatcher: dispatcher} }

func (os *OrderService) ProcessOrder(order *Order) error { // Core business
logic if err := order.Save(); err != nil { return err }

    // Notify everyone who cares
    event := &OrderPlacedEvent{
        order:     order,
        timestamp: time.Now(),
    }

    os.dispatcher.Dispatch(event)

    return nil

}

type Order struct { ID string CustomerEmail string Items []string Total float64
}

func (o \*Order) Save() error { // Save to database fmt.Printf("💾 Saving order
%s\n", o.ID) return nil }

func main() { dispatcher := NewEventDispatcher()

    // Register observers
    dispatcher.Subscribe("order.placed", &EmailNotifier{})
    dispatcher.Subscribe("order.placed", &InventoryUpdater{})
    dispatcher.Subscribe("order.placed", &ShippingLabelCreator{})
    dispatcher.Subscribe("order.placed", &SlackNotifier{})

    orderService := NewOrderService(dispatcher)

    order := &Order{
        ID:            "ORD-123",
        CustomerEmail: "customer@example.com",
        Items:         []string{"Widget", "Gadget"},
        Total:         99.99,
    }

    orderService.ProcessOrder(order)

} Now when you need to add SMS notifications or Webhook calls, you just create a
new observer and register it. The core order processing logic never changes.

Pattern 3: Decorator — Add Behavior Without Inheritance The Problem: You need to
add functionality to existing objects, but Go doesn’t have inheritance, and you
don’t want to modify the original struct.

This is especially useful for middleware-style functionality:

package main

import ( "fmt" "log" "time" ) // Base interface type DataProcessor interface {
Process(data string) string }

// Basic implementation type BasicProcessor struct{}

func (bp \*BasicProcessor) Process(data string) string { return
fmt.Sprintf("processed: %s", data) }

// Decorators that add behavior type LoggingDecorator struct { processor
DataProcessor }

func NewLoggingDecorator(processor DataProcessor) \*LoggingDecorator { return
&LoggingDecorator{processor: processor} }

func (ld \*LoggingDecorator) Process(data string) string {
log.Printf("Processing data: %s", data) result := ld.processor.Process(data)
log.Printf("Processing complete: %s", result) return result }

type TimingDecorator struct { processor DataProcessor }

func NewTimingDecorator(processor DataProcessor) \*TimingDecorator { return
&TimingDecorator{processor: processor} }

func (td \*TimingDecorator) Process(data string) string { start := time.Now()
result := td.processor.Process(data) duration := time.Since(start)
fmt.Printf("⏱️ Processing took %v\n", duration) return result }

type CachingDecorator struct { processor DataProcessor cache map[string]string }

func NewCachingDecorator(processor DataProcessor) \*CachingDecorator { return
&CachingDecorator{ processor: processor, cache: make(map[string]string), } }

func (cd \*CachingDecorator) Process(data string) string { if result, exists :=
cd.cache[data]; exists { fmt.Printf("💾 Cache hit for: %s\n", data) return
result }

    result := cd.processor.Process(data)
    cd.cache[data] = result
    fmt.Printf("💾 Cached result for: %s\n", data)
    return result

}

type RetryDecorator struct { processor DataProcessor maxRetries int }

func NewRetryDecorator(processor DataProcessor, maxRetries int) \*RetryDecorator
{ return &RetryDecorator{ processor: processor, maxRetries: maxRetries, } }

func (rd \*RetryDecorator) Process(data string) string { var result string var
err error

    for i := 0; i <= rd.maxRetries; i++ {
        if i > 0 {
            fmt.Printf("🔄 Retry attempt %d for: %s\n", i, data)
            time.Sleep(time.Millisecond * 100)
        }

        // In a real implementation, you'd handle actual errors
        result = rd.processor.Process(data)
        if result != "" {
            break
        }
    }

    return result

}

func main() { // Start with basic processor processor := &BasicProcessor{}

    // Wrap with decorators - you can mix and match!
    processor = NewLoggingDecorator(processor)
    processor = NewTimingDecorator(processor)
    processor = NewCachingDecorator(processor)
    processor = NewRetryDecorator(processor, 3)

    // Now you have a processor with logging, timing, caching, and retry functionality
    result1 := processor.Process("hello")
    fmt.Printf("Result: %s\n\n", result1)

    // Second call should hit the cache
    result2 := processor.Process("hello")
    fmt.Printf("Result: %s\n", result2)

} The beauty here is that you can compose behaviors. Want caching but not
logging? Just don’t wrap with the logging decorator. Need a different retry
strategy? Create a new retry decorator.

Pattern 4: Adapter — Make Incompatible Interfaces Work Together The Problem: You
have two interfaces that should work together but don’t match.

This happens all the time when integrating with external libraries or legacy
code:

package main

import ( "encoding/json" "encoding/xml" "fmt" )

// Your application expects this interface type NotificationSender interface {
SendNotification(recipient string, message string) error }

// External email service with different interface type EmailService struct {
apiKey string }

func (es \*EmailService) SendEmail(to, from, subject, body string) error {
fmt.Printf("📧 Email sent to %s: %s\n", to, subject) return nil }

// External SMS service with yet another interface type SMSService struct {
accountID string }

func (sms \*SMSService) SendTextMessage(phoneNumber, messageText string, options
map[string]string) error { fmt.Printf("📱 SMS sent to %s: %s\n", phoneNumber,
messageText) return nil }

// External Slack service with completely different interface type SlackWebhook
struct { webhookURL string }

type SlackMessage struct { Text string `json:"text"` Channel string
`json:"channel"` }

func (sw \*SlackWebhook) PostMessage(payload SlackMessage) error { data, \_ :=
json.Marshal(payload) fmt.Printf("💬 Slack message posted: %s\n", string(data))
return nil }

// Legacy system with XML interface type LegacyNotifier struct{}

type XMLNotification struct { XMLName xml.Name `xml:"notification"` To string
`xml:"to"` Message string `xml:"message"` }

func (ln \*LegacyNotifier) SendXMLNotification(xmlData []byte) error {
fmt.Printf("🏛️ Legacy notification sent: %s\n", string(xmlData)) return nil }

// Adapters make everything work with your interface type EmailAdapter struct {
emailService \*EmailService fromAddress string }

func NewEmailAdapter(service *EmailService, from string) *EmailAdapter { return
&EmailAdapter{ emailService: service, fromAddress: from, } }

func (ea \*EmailAdapter) SendNotification(recipient string, message string)
error { return ea.emailService.SendEmail(recipient, ea.fromAddress,
"Notification", message) }

type SMSAdapter struct { smsService \*SMSService }

func NewSMSAdapter(service *SMSService) *SMSAdapter { return
&SMSAdapter{smsService: service} }

func (sa \*SMSAdapter) SendNotification(recipient string, message string) error
{ options := map[string]string{"priority": "high"} return
sa.smsService.SendTextMessage(recipient, message, options) }

type SlackAdapter struct { webhook \*SlackWebhook channel string }

func NewSlackAdapter(webhook *SlackWebhook, channel string) *SlackAdapter {
return &SlackAdapter{ webhook: webhook, channel: channel, } }

func (sa \*SlackAdapter) SendNotification(recipient string, message string)
error { payload := SlackMessage{ Text: fmt.Sprintf("@%s: %s", recipient,
message), Channel: sa.channel, } return sa.webhook.PostMessage(payload) }

type LegacyAdapter struct { legacyNotifier \*LegacyNotifier }

func NewLegacyAdapter(notifier *LegacyNotifier) *LegacyAdapter { return
&LegacyAdapter{legacyNotifier: notifier} }

func (la \*LegacyAdapter) SendNotification(recipient string, message string)
error { xmlNotification := XMLNotification{ To: recipient, Message: message, }

    xmlData, err := xml.Marshal(xmlNotification)
    if err != nil {
        return err
    }

    return la.legacyNotifier.SendXMLNotification(xmlData)

}

// Your application code stays clean type NotificationManager struct { senders
[]NotificationSender }

func NewNotificationManager() \*NotificationManager { return
&NotificationManager{ senders: make([]NotificationSender, 0), } }

func (nm \*NotificationManager) AddSender(sender NotificationSender) {
nm.senders = append(nm.senders, sender) }

func (nm \*NotificationManager) NotifyAll(recipient string, message string) {
for \_, sender := range nm.senders { sender.SendNotification(recipient, message)
} }

func main() { manager := NewNotificationManager()

    // Add different notification methods through adapters
    emailService := &EmailService{apiKey: "email-key"}
    manager.AddSender(NewEmailAdapter(emailService, "noreply@company.com"))

    smsService := &SMSService{accountID: "sms-account"}
    manager.AddSender(NewSMSAdapter(smsService))

    slackWebhook := &SlackWebhook{webhookURL: "https://hooks.slack.com/..."}
    manager.AddSender(NewSlackAdapter(slackWebhook, "#alerts"))

    legacyNotifier := &LegacyNotifier{}
    manager.AddSender(NewLegacyAdapter(legacyNotifier))

    // Send notification through all channels
    manager.NotifyAll("john.doe", "Your order has been processed!")

} Now you can integrate any notification service, regardless of its interface,
just by creating an adapter.

Pattern 5: Composite — Handle Tree Structures Elegantly The Problem: You need to
work with tree-like structures where individual objects and groups of objects
should be treated the same way.

Perfect for file systems, organization charts, UI components, or any
hierarchical data:

package main

import ( "fmt" "strings" )

// Component interface type FileSystemItem interface { GetName() string
GetSize() int Display(indent string) }

// Leaf - individual file type File struct { name string size int }

func NewFile(name string, size int) \*File { return &File{name: name, size:
size} }

func (f \*File) GetName() string { return f.name }

func (f \*File) GetSize() int { return f.size }

func (f \*File) Display(indent string) { fmt.Printf("%s📄 %s (%d bytes)\n",
indent, f.name, f.size) }

// Composite - directory containing files and other directories type Directory
struct { name string items []FileSystemItem }

func NewDirectory(name string) \*Directory { return &Directory{ name: name,
items: make([]FileSystemItem, 0), } }

func (d \*Directory) GetName() string { return d.name }

func (d \*Directory) GetSize() int { totalSize := 0 for \_, item := range
d.items { totalSize += item.GetSize() } return totalSize }

func (d \*Directory) Add(item FileSystemItem) { d.items = append(d.items, item)
}

func (d \*Directory) Remove(itemName string) { for i, item := range d.items { if
item.GetName() == itemName { d.items = append(d.items[:i], d.items[i+1:]...)
break } } }

func (d \*Directory) Display(indent string) { fmt.Printf("%s📁 %s/ (%d bytes
total)\n", indent, d.name, d.GetSize())

    for _, item := range d.items {
        item.Display(indent + "  ")
    }

}

// More complex composite example - UI components type UIComponent interface {
Render() string GetWidth() int GetHeight() int }

type Button struct { text string width int height int }

func NewButton(text string, width, height int) \*Button { return &Button{text:
text, width: width, height: height} }

func (b \*Button) Render() string { return fmt.Sprintf("[%s]", b.text) }

func (b *Button) GetWidth() int { return b.width } func (b *Button) GetHeight()
int { return b.height } type TextBox struct { placeholder string width int
height int }

func NewTextBox(placeholder string, width, height int) \*TextBox { return
&TextBox{placeholder: placeholder, width: width, height: height} }

func (t \*TextBox) Render() string { return fmt.Sprintf("[%s...]",
t.placeholder) }

func (t *TextBox) GetWidth() int { return t.width } func (t *TextBox)
GetHeight() int { return t.height } type Panel struct { name string components
[]UIComponent width int height int }

func NewPanel(name string) \*Panel { return &Panel{ name: name, components:
make([]UIComponent, 0), } }

func (p \*Panel) Add(component UIComponent) { p.components =
append(p.components, component)

    // Recalculate dimensions
    p.recalculateDimensions()

}

func (p \*Panel) recalculateDimensions() { maxWidth := 0 totalHeight := 0

    for _, component := range p.components {
        if component.GetWidth() > maxWidth {
            maxWidth = component.GetWidth()
        }
        totalHeight += component.GetHeight()
    }

    p.width = maxWidth
    p.height = totalHeight

}

func (p \*Panel) Render() string { var parts []string parts = append(parts,
fmt.Sprintf("Panel: %s", p.name))

    for _, component := range p.components {
        parts = append(parts, "  "+component.Render())
    }

    return strings.Join(parts, "\n")

}

func (p *Panel) GetWidth() int { return p.width } func (p *Panel) GetHeight()
int { return p.height }

func main() { fmt.Println("=== File System Example ===")

    // Create file system structure
    root := NewDirectory("root")

    documents := NewDirectory("documents")
    documents.Add(NewFile("resume.pdf", 1024))
    documents.Add(NewFile("letter.docx", 2048))

    photos := NewDirectory("photos")
    photos.Add(NewFile("vacation1.jpg", 4096))
    photos.Add(NewFile("vacation2.jpg", 3072))

    work := NewDirectory("work")
    work.Add(NewFile("project.go", 8192))
    work.Add(NewFile("README.md", 512))

    documents.Add(work) // Nested directory

    root.Add(documents)
    root.Add(photos)
    root.Add(NewFile("config.txt", 256))

    // Display the entire structure
    root.Display("")

    fmt.Printf("\nTotal size: %d bytes\n\n", root.GetSize())

    fmt.Println("=== UI Component Example ===")

    // Create UI structure
    loginForm := NewPanel("Login Form")
    loginForm.Add(NewTextBox("Username", 200, 30))
    loginForm.Add(NewTextBox("Password", 200, 30))
    loginForm.Add(NewButton("Login", 100, 40))

    sidebar := NewPanel("Sidebar")
    sidebar.Add(NewButton("Home", 120, 35))
    sidebar.Add(NewButton("Profile", 120, 35))
    sidebar.Add(NewButton("Settings", 120, 35))

    mainPanel := NewPanel("Main Application")
    mainPanel.Add(sidebar)
    mainPanel.Add(loginForm)

    fmt.Println(mainPanel.Render())
    fmt.Printf("\nMain panel dimensions: %dx%d\n", mainPanel.GetWidth(), mainPanel.GetHeight())

} The composite pattern lets you treat individual items and collections of items
uniformly. Whether you’re working with a single file or an entire directory
tree, the interface is the same.

The Bottom Line These design patterns aren’t just academic exercises — they’re
practical solutions to problems you face every day in Go development:

Strategy eliminates giant switch statements and makes your code extensible
Observer decouples event handling and makes your system more modular Decorator
adds functionality without inheritance and promotes composition Adapter
integrates incompatible interfaces and legacy systems Composite handles
hierarchical data structures elegantly The key insight is that design patterns
in Go often feel more natural than in traditional OOP languages. Go’s
interfaces, composition over inheritance, and explicit design philosophy make
these patterns simpler to implement and understand.

Start with the Strategy pattern — it’s probably the most immediately useful.
Once you see how it cleans up your conditional logic, you’ll start noticing
opportunities for the other patterns throughout your codebase.

Remember: patterns are tools, not rules. Use them when they solve a real
problem, not just because they’re there. Your code should be clearer and more
maintainable after applying a pattern, not more complex.
