Best Practices for Fast, Comprehensive Crawling Without Getting Blocked

Websites and CDNs often employ anti-scraping measures (rate limiting, IP blocks,
bot detection) that can hinder a crawler’s completeness. Below are lessons from
the field and detailed techniques to make your Go/Colly crawler both fast and
stealthy. These go beyond general concepts – they are actionable changes you can
apply to avoid throttles and blockers.

Throttle Your Request Rate and Concurrency

Don’t overload the target site. Many sites will throttle or block clients that
send too many requests too quickly (e.g. returning HTTP 429 “Too Many Requests”
or even temporarily banning your IP) webscraping.ai . To prevent this:

Use Colly’s Rate Limiting: Configure a delay between requests. For example, use
collector.Limit(&colly.LimitRule{}) to set a max frequency (e.g. 1 request per
second) with some random jitter webscraping.ai . A small random delay makes
traffic look less like a rapid-fire bot. Colly supports rules per domain – you
can even set slower limits for “sensitive” domains if needed webscraping.ai
webscraping.ai .

Limit Parallel Requests per Domain: High concurrency from one IP can trigger
defenses. Even if your crawler is asynchronous, cap the parallelism for each
host. For instance, set Parallelism: 1 (or a low number) in the Colly LimitRule
for the target domain webscraping.ai . This ensures requests to the same site
are essentially serial with a delay, avoiding bursts.

Implement Backoff Retries: Integrate an exponential backoff when you do hit a
rate-limit. In Colly’s OnError, detect status 429/503 and pause before retrying.
For example, if you get a 429, wait (e.g. sleep 2^n seconds) then retry the
request instead of hammering again immediately webscraping.ai . This backoff
gives the server/CDN time to cool down and stops your crawler from getting
outright banned.

Monitor and Adjust: Keep counters of requests and errors. If you notice error
rates spiking (e.g. lots of 429/403 responses), automatically slow down further
webscraping.ai . Adapting your crawl rate in real-time helps maintain access –
essentially an auto-throttle that reacts to site feedback.

Rotate IP Addresses and Distribute Load

Avoid a single-IP bottleneck. If all crawl traffic comes from one IP address,
it’s easy for the target to identify and block that IP webscraping.ai . Mitigate
this by spreading requests across multiple IPs:

Use Proxies or VPNs: Colly allows setting a proxy function to route requests.
You can supply a pool of proxy servers (datacenter or residential) and rotate
them so each request (or each batch of requests) uses a different exit IP
webscraping.ai webscraping.ai . For example, Colly’s
proxy.RoundRobinProxySwitcher can cycle through a list of proxy URLs on each
request webscraping.ai . This rotation prevents any single IP from hitting too
many pages.

Multiple Instances / Regions: Since you’re on Fly.io, consider deploying
crawlers in multiple regions or using multiple Fly instances. Each instance will
have a different IP. You can partition the crawl (e.g. split the URL list or
site sections among instances) so that no one IP does the entire crawl. This
achieves high aggregate speed without overwhelming from one address.

Handle Proxy Throttling: If using proxies, treat each proxy IP with the same
care – apply per-proxy/domain rate limits. Also implement logic to detect a
“bad” or blocked proxy (e.g. repeated errors) and swap it out. The goal is to
always have some IP able to fetch while others back off.

CDN Specific Tip – Origin Fetch: In some cases (especially with Cloudflare), an
advanced trick is to find the website’s origin server IP (bypassing the CDN).
This can sometimes let you crawl without Cloudflare seeing you. Use with
caution: origin IPs may be hidden or have their own protections. But if
available, it removes the CDN’s bot detection from the equation scrapeops.io .

Use Realistic Headers and User-Agent Strings

Don’t announce yourself as a bot. Many sites block clients with missing or
suspicious headers (like the default Go user-agent). You already use a custom UA
– ensure it truly mimics a browser. Key steps:

Rotate Authentic User-Agents: Avoid using one constant string or anything
identifying as a crawler. Gather a list of real browser UA strings (Chrome,
Firefox on various OSes) and rotate them for each request webscraping.ai
webscraping.ai . Colly’s OnRequest callback is a good place to pick a random UA
per request webscraping.ai . This makes it harder for a site to flag your
traffic by a static fingerprint.

Populate All Typical Headers: Beyond User-Agent, set other headers that a normal
browser would send. This includes Accept (e.g. text/html), Accept-Language (e.g.
en-US), Accept-Encoding (gzip, deflate if you handle compression), and
Connection: keep-alive webscraping.ai . Having a full header set makes your
requests look more legitimate and less like a bare script.

Use Referer When Crawling: As your crawler follows links, set the Referer header
to the page it came from (except for the first page). Real users always send a
Referer when clicking links. You can do this in Colly by tracking the current
page URL – for example, in OnRequest use r.Headers.Set("Referer",
previousPageURL) or even r.Headers.Set("Referer", e.Request.URL.String()) in an
OnHTML callback before visiting the link webscraping.ai . This small detail
helps bypass simple bot detections that check for missing referers.

Beware of TLS/HTTP Fingerprints: Modern bot defenses (like Cloudflare Bot
Manager) examine low-level client fingerprints (TLS cipher suites, HTTP/2
sequence, etc.) that the Go HTTP client might expose as non-browser. There’s no
easy Colly fix for this, but being aware is important. In practice, using a
diverse set of proxies and perhaps different HTTP client configurations can
slightly vary these fingerprints. If a particular site is still flagging you, it
may be due to these factors – at which point using a headless browser (which has
a real browser fingerprint) might be the only solution (more on this below).

Simulate Human-Like Crawling Behavior

Make your access pattern less bot-like. Anti-scraping systems not only look at
volume, but also patterns. If your crawler behaves too mechanically, it raises
red flags. Strategies to humanize it:

Randomize Timing: We mentioned adding random delay between requests – this is
crucial. Mix short and slightly longer pauses so the intervals aren’t perfectly
consistent. For example, add RandomDelay in Colly’s limit rules or even a
time.Sleep(rand(500ms-2s)) in the OnRequest callback webscraping.ai
webscraping.ai . This mimics how real users browse unevenly (reading some pages
longer than others, etc.).

Depth-Based Pauses: If you’re doing a deep crawl through links, consider pausing
a bit more for deeper page levels. The idea is that a user might spend more time
as they navigate further. The Colly snippet below demonstrates adding a delay if
r.Depth > 1 webscraping.ai . While not strictly necessary, touches like this can
reduce the chance of detection by behavioral analysis.

Avoid Monotonic URL sequences: If you’re crawling numeric or alphabetical
sequences (say page=1,2,3... or a list of IDs), try not to fetch them in perfect
order at top speed. If possible, crawl in a slightly randomized order or insert
other page fetches in between. Bots that iterate sequentially are easier to
spot.

Fetch Supporting Resources (if feasible): Some advanced bot detectors check if
you load images, CSS, or JS files that a normal browser would request. Pure
HTML-only fetching can mark you as a bot. If a particular site is blocking you
and you suspect this is why, you could have your crawler occasionally fetch a
key resource (like an image or CSS) after the page, then discard it. This
increases load, so use sparingly. (Only do this if needed; it’s an overhead vs.
benefit trade-off.)

Use Sessions & Cookies Like a Browser: Enable a cookie jar so that your crawler
maintains cookies set by the site webscraping.ai . This means your requests will
carry session cookies, which is what browsers do. If your crawler starts a fresh
session on every request (no cookies), some sites detect that every request
looks like a new “user” (which is abnormal). By reusing cookies, you appear as
one consistent visitor, which is more natural as you crawl the site.

Handle JavaScript-Heavy Sites and Bot Challenges

JavaScript and bot-challenges can blind a simple crawler. If a site loads
critical content via JS or uses a service like Cloudflare to challenge visitors,
Colly alone will struggle since it can’t run JS webscraping.ai . To tackle this:

Integrate a Headless Browser for JS: For pages that require rendering (SPA apps,
infinite scroll, or Cloudflare “5-second” challenges), incorporate a headless
browser (e.g. Chrome via Puppeteer or Playwright for Go). You can automate it to
fetch the page, execute any JS, then pass the HTML to Colly for parsing. This
hybrid approach lets you use Colly’s fast parsing on the results of a real
browser fetch webscraping.ai webscraping.ai . There are Go libraries like
chromedp that can control Chrome in-headless mode, or you can use an external
render service. Use this only for pages where Colly gets stuck – it’s
slower/heavier, so maybe detect a Cloudflare challenge or missing content and
then fall back to headless for those URLs.

Bypass Cloudflare and WAFs: If you know certain sites are behind aggressive WAFs
(Cloudflare, Akamai, etc.), you might choose to use scraping APIs for those.
Services like ScrapingBee, ZenRows, etc., handle Cloudflare bypass (using
browser tech and proxy pools) for you – you just make an API call and get the
page HTML. This can be a quick remedy for a few problematic domains without
reinventing the wheel. (Cite: these services combine headless Chrome + rotating
residential proxies to solve challenges for you.)

CAPTCHA Handling: Traditional CAPTCHAs (reCAPTCHA, hCaptcha, etc.) are nearly
impossible to solve with Colly alone, by design. If your crawl hits a CAPTCHA
wall, you have a few choices webscraping.ai : (1) Use a CAPTCHA-solving service
API to solve and submit the response (adds cost and complexity); (2) Skip or
defer those pages (often if you slow down or change IP, the site might not
present the CAPTCHA on a second try); or (3) as a last resort, embed a pause for
manual intervention (not feasible for large crawls, but mentioning it). In
general, if you’ve reached a CAPTCHA, it means the site has seriously flagged
you – it’s better to adjust your strategy earlier (slower rate, new IP, etc.) to
avoid getting to that stage.

Manage Sessions and Cookies

Maintain state like a real user. Using a single IP with multiple rapid
connections can look suspicious, but also not maintaining a session is
suspicious in other ways. Here’s how to manage session state:

Enable Cookie Jar: Colly supports using a cookie jar (via the net/http
cookiejar). Activate this so that any cookies set in responses are saved and
sent on subsequent requests webscraping.ai . This is important for sites that
use login sessions or even for those that set a cookie to mark that you passed a
Cloudflare challenge or consent banner. Maintaining these cookies can prevent
duplicate challenges and makes your crawler’s journey look like a continuous
browsing session rather than unrelated one-off visits.

Simulate Login if Needed: If parts of the site are behind login and you have
credentials or a way to log in, perform that step with Colly (or a browser)
first to obtain an auth session cookie, then crawl with that session. Logging in
via Colly might require handling form tokens, etc., but it ensures you’re
recognized as an authenticated user. This can also sometimes bypass certain rate
limits applied only to anonymous visitors. (Only applicable if your use-case
involves logging in; otherwise skip.)

Avoid Losing Session on Redirects: Ensure you don’t inadvertently drop cookies
on redirects. The default HTTP client will carry them if the jar is enabled, but
if you manually handle redirects or use multiple collectors, make sure to share
the cookie jar among them if hitting the same domain, so you carry over those
cookies and appear consistent.

Robust Error Handling and Adaptive Crawling

Expect errors and adapt to them. Incorporate a smart error-handling mechanism so
your crawler can react to blocking in real time and adjust course:

Centralize Error Handling: Use collector.OnError to catch HTTP errors for any
request webscraping.ai . Check the status code and reason in that callback. This
is your chance to implement the logic: e.g., on 429 or 503, trigger a backoff as
discussed; on 403 or 401, assume the request was denied – you might then switch
to a different proxy or user-agent and retry webscraping.ai . By coding these
rules, you make the crawler resilient – it won’t just give up or keep spamming,
it will respond appropriately.

Switch Tactics on High Error Rates: If you notice a high percentage of requests
failing (e.g. >10% of pages getting blocked), consider automatically dialing
down the crawl or changing strategy webscraping.ai . This could mean reducing
concurrency further, increasing delays, or injecting a longer “cool-off” pause
after a batch of pages. It could also mean picking a fresh proxy or a new batch
of proxies if you suspect the whole pool got flagged. Essentially, treat an
elevated error rate as an alarm to be more gentle.

Logging and Monitoring: Keep logs of your crawling speed and responses. Track
how fast you’re hitting each site and how often you get errors or blocks. Over
time you’ll identify patterns (“Site X starts blocking after 50 pages in 30
seconds” or “Site Y allows rapid calls to some endpoints but not others”). Use
this data to fine-tune domain-specific rules – for instance, you might crawl one
site slower than another because you know it’s more sensitive.

Test in Isolation: When you face issues with a particular site, try crawling
that site alone with various settings (different user agents, slower speeds,
etc.) to see what triggers the block. This experimentation can yield specific
insights (maybe a certain cookie or header is needed, or a certain URL pattern
trips a WAF rule). Incorporate those findings back into your main crawler
configuration.

Additional Considerations

Finally, a few extra best practices to ensure comprehensive crawls:

Respect robots.txt: Even though it’s not a technical anti-bot mechanism,
following robots.txt rules is good etiquette and sometimes legally important.
Colly can be set to respect robots.txt by collector.IgnoreRobotsTxt = false
webscraping.ai . This includes observing any Crawl-delay directive if present,
which directly helps you avoid being blocked by crawling at a site-approved
rate.

Use Sitemaps and Known URL Patterns: To get a comprehensive crawl, leverage a
site’s sitemap.xml if available. Fetch it first to obtain a list of URLs. This
can reduce the need for exhaustive crawling and minimize hits on HTML pages just
to discover links. Fewer requests for discovery means you can allocate more
budget to the important pages, reducing load on the site and chances of
blocking.

Distributed Crawling: If the target website is very large, consider a
distributed approach (multiple machines or processes). For example, split the
URL list alphabetically or by site section, and have different workers (with
different IPs) crawl in parallel. This not only speeds up the crawl but also
sidesteps single-IP limits. Just ensure each worker still follows the politeness
rules for its share.

Stay Updated on Anti-Bot Trends: Anti-scraping measures evolve. For instance,
new browser fingerprinting techniques or stricter WAF rules may require
adjustments. Keep an eye on communities or updates (e.g., Cloudflare changes,
new bot detection libraries) so you can adapt your crawler. Sometimes small
changes in how you handle TLS, HTTP protocols, or JavaScript execution can make
a big difference in staying under the radar.

By applying these techniques, you should see more consistent results across
different websites and CDN protections. In practice, it’s about finding the
right balance: fast and comprehensive, yet polite and camouflaged. Start with
conservative settings (to avoid blocks), then gradually increase crawl speed in
controlled ways to find the maximum safe throughput for each target. Using the
strategies above – from throttling and rotating IPs to mimicking real browsers –
will significantly improve your crawler’s success rate in fetching all pages
without interruptions.

Sources: Some approaches above are informed by real-world web scraping practices
and Colly-specific guidance, including handling rate limits webscraping.ai
webscraping.ai , using proxies webscraping.ai webscraping.ai , rotating
user-agents and headers webscraping.ai webscraping.ai , dealing with JS and
CAPTCHA challenges webscraping.ai webscraping.ai , managing cookies/sessions
webscraping.ai , and implementing adaptive error-handling/backoff strategies
webscraping.ai webscraping.ai . These will help ensure your Go-based crawler
remains efficient and gets the full picture of the site without getting shut
out.
