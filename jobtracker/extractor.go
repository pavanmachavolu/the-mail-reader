package jobtracker

import (
	"regexp"
	"strings"
)

var (
	// Regex for job-related keywords in subject or body
	jobKeywordsRegex = regexp.MustCompile(`(?i)\b(career|careers|application|applications|recruiting|recruitment|hiring|interview|interviews|talent acquisition|job opportunity|job offer|candidate|referral|referrals|referred)\b`)

	// Job ID patterns like "Req: 12345", "Job ID: R000106137", "Ref #1234", "REQ-9988", "R000123"
	jobIDRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:requisition|req|job\s*id|ref(?:erence)?|job\s*#|req\s*#)\b[:#\s]*([A-Za-z0-9_-]{3,25})`),
		regexp.MustCompile(`\b(R\d{6,}[A-Za-z0-9]*)\b`),
		regexp.MustCompile(`\b(REQ[-_]\d+)\b`),
	}

	// Team patterns like "(Consumer Team)", "Inference Team", "Infrastructure Department"
	teamRegexes = []*regexp.Regexp{
		regexp.MustCompile(`\(([^)]+Team)\)`),
		regexp.MustCompile(`(?i)\b([A-Za-z0-9&/]+(?:\s+[A-Za-z0-9&/]+)?\s+(?:Team|Department|Division|Org))\b`),
	}

	// Status transition patterns
	offerPatterns = []string{
		"pleased to offer",
		"delighted to offer",
		"offer of employment",
		"formal job offer",
		"congratulations on your offer",
		"offer letter",
	}

	interviewPatterns = []string{
		"invitation to interview",
		"invite you to interview",
		"interview with",
		"interview confirmation",
		"screening invitation",
		"screening is ready",
		"complete your agoda screening",
		"complete your screening",
		"screening call",
		"screening assessment",
		"phone screen",
		"technical screen",
		"code challenge",
		"coding challenge",
		"technical assessment",
		"take home assignment",
		"take-home assignment",
		"codesignal",
		"hackerrank",
		"schedule a call",
		"schedule an interview",
		"chat with the team",
		"speak with the hiring manager",
	}

	rejectionPatterns = []string{
		"not moving forward",
		"will not be moving forward",
		"will not be taking your application",
		"not be taking your application forward",
		"decided to pursue other",
		"decided not to proceed",
		"decided not to move forward",
		"decided to move forward with other",
		"regret to inform",
		"other candidates",
		"other applicants",
		"unfortunately",
		"will not be progressing",
		"won't be progressing",
		"won't be moving forward",
		"move forward with candidates whose",
		"moving forward with candidates whose",
		"decided to move forward with candidates",
		"position has been filled",
		"thank you for your interest",
	}

	referralPatterns = []string{
		"has referred you",
		"have been referred",
		"you've been referred",
		"you have been referred",
		"referred you for",
		"referred you to",
		"referred for a role",
		"referred to a role",
		"candidate referral",
		"employee referral",
		"referral confirmation",
		"application referral",
		"complete referral application",
		"wants to work with you",
		"thinks you’d be great",
		"thinks you'd be great",
		"thinks you would be great",
		"referred to us by",
		"referred you as a candidate",
		"someone in your network has referred you",
		"applying via a referral",
	}

	underReviewPatterns = []string{
		"update on your application",
		"application update",
		"an update regarding your",
		"under review",
		"reviewing your application",
		"currently reviewing applications",
		"next steps",
		"status of your application",
	}

	appliedPatterns = []string{
		"thank you for applying",
		"thanks for applying",
		"application received",
		"received your application",
		"application has been received",
		"has been received",
		"application confirmation",
		"confirming your application",
		"we have received your application",
		"we have received",
		"applied to our",
		"applied to the",
		"applied for the",
		"applied for",
		"application to join",
		"application to the",
		"application to",
	}
)

// IsJobRelated evaluates whether an email belongs to a recruiting / job search context.
func IsJobRelated(msg *EmailMessage) bool {
	subjectLower := strings.ToLower(msg.Subject)
	fromLower := strings.ToLower(msg.From)

	// Exclude false positive sender domains (newsletters, immigration portals, surveys, delivery failure notices)
	if strings.Contains(fromLower, "pointer.io") ||
		strings.Contains(fromLower, "qualtrics-survey.com") ||
		strings.Contains(fromLower, "@nic.in") ||
		strings.Contains(fromLower, "postmaster@") {
		return false
	}
	if strings.HasPrefix(subjectLower, "undeliverable:") {
		return false
	}

	// Exclude false positive subjects (loans, credit cards, newsletters, app stores, surveys)
	falsePositives := []string{
		"credit card", "loan application", "mortgage", "app store", "play store",
		"software update", "mobile application", "banking", "statement", "receipt",
		"payment", "invoice", "verification code", "security code", "flight", "hotel", "newsletter",
		"online loan", "preferred banking", "loan with", "dream job",
		"fti-ttp", "uk visas and immigration", "recruiting process survey",
		"candidate experience survey", "demographic survey", "rate our recruiting",
		"points", "proposal", "need help in estimates", "big read e-book",
		"please add me to your linkedin network", "sorry to see you go",
		"we want to know what you think",
	}
	for _, fp := range falsePositives {
		if strings.Contains(subjectLower, fp) {
			return false
		}
	}

	// For personal email domains, require clear recruiting/referral signal in subject
	isPersonalDomain := strings.HasSuffix(fromLower, "@gmail.com") ||
		strings.HasSuffix(fromLower, "@live.com") ||
		strings.HasSuffix(fromLower, "@outlook.com") ||
		strings.HasSuffix(fromLower, "@hotmail.com") ||
		strings.HasSuffix(fromLower, "@yahoo.com")
	if isPersonalDomain {
		hasJobSignal := strings.Contains(subjectLower, "referral") ||
			strings.Contains(subjectLower, "referred") ||
			strings.Contains(subjectLower, "interview") ||
			strings.Contains(subjectLower, "opportunity") ||
			strings.Contains(subjectLower, "hiring") ||
			strings.Contains(subjectLower, "application") ||
			strings.Contains(subjectLower, "candidate") ||
			strings.Contains(subjectLower, "role") ||
			strings.Contains(subjectLower, "position") ||
			strings.Contains(subjectLower, "profile") ||
			strings.Contains(subjectLower, "job")
		if !hasJobSignal {
			return false
		}
	}

	// Check if sender address contains recruiting hints
	recruitingSender := strings.Contains(fromLower, "recruiting") ||
		strings.Contains(fromLower, "careers") ||
		strings.Contains(fromLower, "jobs") ||
		strings.Contains(fromLower, "talent") ||
		strings.Contains(fromLower, "greenhouse") ||
		strings.Contains(fromLower, "lever") ||
		strings.Contains(fromLower, "workday") ||
		strings.Contains(fromLower, "ashby") ||
		strings.Contains(fromLower, "makipeople") ||
		strings.Contains(fromLower, "workable") ||
		strings.Contains(fromLower, "breezy") ||
		strings.Contains(fromLower, "recruitee") ||
		strings.Contains(fromLower, "teamtailor") ||
		strings.Contains(fromLower, "smartrecruiters") ||
		strings.Contains(fromLower, "applytojob")

	if recruitingSender {
		return true
	}

	// Check subject and body keywords
	if jobKeywordsRegex.MatchString(msg.Subject) {
		return true
	}

	combinedBody := msg.BodyText + " " + msg.BodyMarkdown
	if len(combinedBody) > 4000 {
		combinedBody = combinedBody[:4000]
	}

	return jobKeywordsRegex.MatchString(combinedBody)
}

// ResolveActualCompany resolves the real hiring company name when emails are sent by
// third-party ATS platforms, assessment providers, or email relays (e.g. ashbyhq.com -> DualEntry, makipeople.com -> Agoda).
func ResolveActualCompany(domain, defaultCompany, subject, body string) string {
	d := strings.ToLower(domain)

	vendorPlatforms := []string{
		"makipeople.com", "greenhouse.io", "greenhouse-mail.io", "greenhouse-jobs.com",
		"lever.co", "ashbyhq.com", "myworkday.com",
		"smartrecruiters.com", "workablemail.com", "workable.com", "hackerrank.com",
		"codesignal.com", "hirevue.com", "testgorilla.com", "applytojob.com", "vssend.com",
		"teamtailor.com", "teamtailor-mail.com", "jobvite.com",
		"breezy-mail.com", "breezy.hr", "comeet-notifications.com", "comeet.com",
		"recruitee.com", "recruiterbox.com", "taleo.com", "taleo.net",
		"invalidemail.com",
	}

	isVendor := false
	for _, v := range vendorPlatforms {
		if d == v || strings.HasSuffix(d, "."+v) {
			isVendor = true
			break
		}
	}

	defLower := strings.ToLower(defaultCompany)
	// Also treat defaultCompany or domain containing vendor keywords as vendor
	if strings.Contains(defLower, "ats") ||
		strings.Contains(defLower, "ashby") ||
		strings.Contains(defLower, "greenhouse") ||
		strings.Contains(defLower, "lever") ||
		strings.Contains(defLower, "workable") ||
		strings.Contains(defLower, "breezy") ||
		strings.Contains(defLower, "teamtailor") ||
		strings.Contains(defLower, "recruitee") ||
		strings.Contains(defLower, "smartrecruiters") ||
		strings.Contains(defLower, "applytojob") ||
		strings.Contains(defLower, "candidates") ||
		strings.Contains(defLower, "hire") ||
		strings.Contains(defLower, "message") ||
		strings.Contains(defLower, "inbound") ||
		strings.Contains(defLower, "application status") ||
		strings.Contains(d, "greenhouse") ||
		strings.Contains(d, "ashby") ||
		strings.Contains(d, "lever") ||
		strings.Contains(d, "workable") ||
		strings.Contains(d, "smartrecruiters") ||
		strings.Contains(d, "teamtailor") ||
		strings.Contains(d, "myworkday") ||
		strings.Contains(d, "makipeople") ||
		strings.Contains(d, "breezy") ||
		strings.Contains(d, "comeet") ||
		strings.Contains(d, "recruitee") ||
		strings.Contains(d, "recruiterbox") ||
		strings.Contains(d, "taleo") ||
		strings.Contains(d, "applytojob") ||
		strings.Contains(d, "invalidemail") {
		isVendor = true
	}

	isPersonalRelay := strings.Contains(d, "gmail.com") ||
		strings.Contains(d, "live.com") ||
		strings.Contains(d, "outlook.com") ||
		strings.Contains(d, "hotmail.com") ||
		strings.Contains(d, "yahoo.com")

	// Direct specialized domain mappings
	if d == "customer.io" || strings.HasSuffix(d, ".customer.io") {
		return "Customer.io"
	}
	if strings.Contains(d, "invalidemail.com") {
		if strings.Contains(strings.ToLower(subject+" "+body), "barclays") || strings.Contains(d, "barclays") {
			return "Barclays"
		}
		if strings.Contains(strings.ToLower(subject+" "+body), "american express") || strings.Contains(strings.ToLower(subject+" "+body), "amex") {
			return "American Express"
		}
	}
	if strings.Contains(d, "taleo.com") || strings.Contains(d, "taleo.net") {
		if strings.Contains(strings.ToLower(subject+" "+body), "dbs") {
			return "DBS Bank"
		}
	}
	if strings.Contains(d, "myworkday.com") {
		for _, w := range []string{"oliver wyman", "salesforce", "adobe", "expedia", "grab", "illumina", "mastercard", "warnerbros", "morgan stanley", "bank of america"} {
			if strings.Contains(strings.ToLower(subject+" "+body), w) {
				return formatTitle(w)
			}
		}
	}
	if strings.Contains(strings.ToLower(subject+" "+body), "servicenow") {
		return "ServiceNow"
	}
	if strings.Contains(strings.ToLower(subject+" "+body), "morgan stanley") || strings.HasPrefix(d, "ms@") {
		return "Morgan Stanley"
	}
	if strings.Contains(strings.ToLower(subject+" "+body), "bank of america") || strings.HasPrefix(d, "ghr@") {
		return "Bank of America"
	}
	if strings.Contains(d, "gs.com") || strings.Contains(strings.ToLower(subject), "goldman sachs") {
		return "Goldman Sachs"
	}
	if strings.Contains(d, "mlp.com") || strings.Contains(strings.ToLower(subject+" "+body), "millennium") {
		return "Millennium"
	}
	if strings.Contains(strings.ToLower(subject), "at picnic") || strings.Contains(strings.ToLower(subject), "@ picnic") || strings.Contains(strings.ToLower(subject), "to picnic") {
		return "Picnic"
	}

	// 1. Scan subject patterns (highest accuracy)
	subjectPatterns := []*regexp.Regexp{
		// Oliver Wyman / Workday subject formats
		regexp.MustCompile(`(?i)(?:Application status update|Status update|Application update)\s*[-:]\s*(?:[A-Z0-9_-]+\s+)?([A-Za-z0-9&.\s]{2,30}?)\s*[-–—:]`),
		regexp.MustCompile(`(?i)\bat\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+team|$)\s*$`),
		// SmartRecruiters notifications
		regexp.MustCompile(`(?i)^([A-Za-z0-9&.\s]{2,25}?)\s+(?:has received your application|viewed your application|has viewed your application)`),
		// Background check forms
		regexp.MustCompile(`(?i)(?:Your Background Forms for|Background Forms for)\s+([A-Za-z0-9&.,\s]{2,30}?)(?:,\s*Inc|\s+Have Been)`),
		// Agoda screening reminder
		regexp.MustCompile(`(?i)Reminder:\s*(?:\d+\s+days\s+left\s+to\s+complete\s+your\s+)?([A-Za-z0-9&.\s]{2,25}?)\s+screening`),
		// BCG Digital Ventures consent
		regexp.MustCompile(`(?i)(?:Confirming consent to remain in)\s+([A-Za-z0-9&.\s]{2,30}?)(?:'s|'\s+candidate|\s+candidate)`),
		// Feedback from / GDPR from
		regexp.MustCompile(`(?i)(?:Feedback from|GDPR Notification from)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|$)`),
		// Excited to hear you're interested in Company!
		regexp.MustCompile(`(?i)interested in\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|$)`),
		// Palantir - Contact for Future Job Opportunities / Qonto - Thanks for submitting
		regexp.MustCompile(`(?i)^([A-Za-z0-9&.\s]{2,20}?)\s*-\s*(?:Contact for Future Job Opportunities|Thanks for submitting)`),
		// Profile submitted to SumTotal
		regexp.MustCompile(`(?i)Profile submitted to\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|$)`),
		// Resume Received for ... at Risk Management Solutions (RMS)
		regexp.MustCompile(`(?i)(?:Resume Received for\s+.*?at)\s+([A-Za-z0-9&.()\s]{2,35}?)(?:!|\.|\r|\n|$)`),
		// Interview / discussion at Company
		regexp.MustCompile(`(?i)(?:discussion at|interview at)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|$)`),
		// Bracketed prefix: [Qonto] Your application at Qonto
		regexp.MustCompile(`^\[([A-Za-z0-9&.\s]{2,20}?)\]`),
		// Thanks for your interest in Company
		regexp.MustCompile(`(?i)(?:Thanks for your interest in|Thank you for your interest in)\s+([A-Za-z0-9&.,\s]{2,25}?)(?:,\s*[A-Za-z]+|\s+[-–—]|!|\.|\r|\n|$)`),
		// Thank you from Company
		regexp.MustCompile(`(?i)Thank you from\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|$)`),
		// Application to Company
		regexp.MustCompile(`(?i)Your Application to\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|$)`),
		// Thanks for applying to Company
		regexp.MustCompile(`(?i)(?:thank you for applying to|thanks for applying to|thank you for applying with|thanks for applying with|thank you for applying at|thanks for applying at|thank you for your application to|thanks for your application to|yes, you applied to)\s+([A-Za-z0-9&.,:\s]{2,30}?)(?:!|\.|\r|\n|,|\s+team|\s+-|\s+for|$)`),
		// Referrals
		regexp.MustCompile(`(?i)(?:referred to a role at|referred to a role in|referred for a role at|referred you to|referred to)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+for|\s+regarding|$)`),
		regexp.MustCompile(`(?i)(?:referral\s*@|referral\s*at|referral\s*for)\s*([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+Hyd|\s+blr|$)`),
		regexp.MustCompile(`(?i)^([A-Za-z0-9&.]{2,20}?)\s+referral`),
		regexp.MustCompile(`(?i)(?:Opportunity\s*\|\s*|opportunity with\s+)([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|$)`),
		regexp.MustCompile(`(?i)(?:thinks you’d be great for|thinks you'd be great for|great for)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+and|\s+we'd|$)`),
		regexp.MustCompile(`(?i)(?:opportunity at)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+-|\s+the|$)`),
		regexp.MustCompile(`(?i)(?:role at|position at)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+team|$)`),
		regexp.MustCompile(`(?i)(?:Regarding your application to|Regarding your application at)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|$)`),
		regexp.MustCompile(`(?i)(?:Your journey @|Your journey at)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+starts|$)`),
		regexp.MustCompile(`(?i)(?:interest in joining|interest in|Interest in)\s+(?:the\s+)?([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+and|\s+[-–—]|$)`),
		regexp.MustCompile(`(?i)(?:Application Update from|Update from|application with|application at|application to)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+team|\s+group|$)`),
		regexp.MustCompile(`(?i)(?:interview with|interview at|speaking with)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+for|\s+regarding|\s+job|\s+team|$)`),
		regexp.MustCompile(`(?i)^([A-Za-z0-9&.]{2,20}(?:\s+[A-Za-z0-9&.]{2,20})?)\s*[-|–—:|]\s*(?:We've got your|Confirmation|Application|Follow-up|Update|Thank you|Your)`),
		regexp.MustCompile(`(?i)^([A-Za-z0-9&.]{2,20}(?:\s+[A-Za-z0-9&.]{2,20})?)\s+(?:Application Update|Job Application|Update)`),
		regexp.MustCompile(`(?i)Your\s+([A-Za-z0-9&.\s]{2,20}?)\s+Application`),
		regexp.MustCompile(`(?i)Application Confirmation\s*[-–—:]\s*([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|$)`),
		regexp.MustCompile(`(?i)(?:Update on\s+)([A-Za-z0-9&.\s]{2,25}?)\s+Application`),
		regexp.MustCompile(`(?i)^[A-Za-z0-9&/,\s()-]+\s+[-–—]\s+([A-Za-z0-9&.\s]{2,25}?)$`),
	}

	for _, re := range subjectPatterns {
		if m := re.FindStringSubmatch(subject); len(m) > 1 {
			comp := cleanExtractedCompany(m[1])
			if isValidExtractedCompany(comp) {
				return comp
			}
		}
	}

	// 2. Subdomain check for ATS vendors (e.g. framer.breezy-mail.com -> Framer, kaizo.recruitee.com -> Kaizo, victoriametrics.na.teamtailor-mail.com -> VictoriaMetrics)
	genericSubdomains := map[string]bool{
		"candidates": true, "hire": true, "inbound": true, "message": true,
		"conversations": true, "login": true, "na": true, "eu": true, "us": true,
		"app": true, "mail": true, "email": true, "no-reply": true, "noreply": true,
		"notifications": true, "auth": true, "post": true, "api": true, "jobs": true,
		"careers": true, "talent": true, "recruiting": true, "system": true, "reply": true,
		"service": true, "support": true, "www": true, "m": true, "events": true,
	}

	if isVendor {
		for _, v := range vendorPlatforms {
			if strings.HasSuffix(d, "."+v) {
				sub := strings.TrimSuffix(d, "."+v)
				parts := strings.Split(sub, ".")
				for i := len(parts) - 1; i >= 0; i-- {
					subPart := strings.ToLower(parts[i])
					subPartClean := strings.TrimRight(subPart, "0123456789")
					if subPartClean == "" {
						subPartClean = subPart
					}
					if !genericSubdomains[subPart] && !genericSubdomains[subPartClean] && len(subPartClean) >= 3 {
						return formatTitle(subPartClean)
					}
				}
			}
		}
	}

	// 3. Scan body patterns
	bodyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:doing here at|work here at|working here at|work we are doing here at)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+-|:)`),
		regexp.MustCompile(`(?i)(?:interest in joining our team at|joining our team at|interest in joining us here at|joining us here at|us here at|joining us at|joining)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+-|\s+we're|\s+for|\s+is|\.|\s+Now|\s+We)`),
		regexp.MustCompile(`(?i)(?:applying for the|applied for the|applying to the|applied to the)\s+(?:.*?)\s+(?:role at|position at|job at|at)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,)`),
		regexp.MustCompile(`(?i)(?:role here at|position here at|team here at|position at|role at)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+-|\s+we're|\s+for|\s+is)`),
		regexp.MustCompile(`(?i)(?:thank you for applying to|thanks for applying to|thank you for applying with)\s+([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+team|\s+-|\s+for|$)`),
		regexp.MustCompile(`(?i)(?:interest in joining|interest in|Interest in)\s+(?:the\s+)?([A-Za-z0-9&.\s]{2,25}?)(?:!|\.|\r|\n|,|\s+and|$)`),
	}

	for _, re := range bodyPatterns {
		if m := re.FindStringSubmatch(body); len(m) > 1 {
			comp := cleanExtractedCompany(m[1])
			if isValidExtractedCompany(comp) {
				return comp
			}
		}
	}

	// Specific template fallback for Ashby emails
	if strings.Contains(body, "three (3) rounds") && strings.Contains(body, "TALENT TEAM") {
		return "Bjak"
	}

	// If domain is not an ATS/vendor or generic relay and has a valid defaultCompany, keep it!
	if !isVendor && !isPersonalRelay && defaultCompany != "" &&
		!strings.EqualFold(defaultCompany, "Email") &&
		!strings.EqualFold(defaultCompany, "Mail") &&
		!strings.EqualFold(defaultCompany, "Hire") &&
		!strings.EqualFold(defaultCompany, "Candidates") &&
		!strings.EqualFold(defaultCompany, "Message") &&
		!strings.EqualFold(defaultCompany, "Inbound") &&
		!strings.EqualFold(defaultCompany, "Application status") {
		return defaultCompany
	}

	if isVendor {
		return "Unknown Employer"
	}
	if isPersonalRelay {
		return "Personal Referral"
	}
	return HeuristicCompanyName(domain)
}

func cleanExtractedCompany(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "-–—,.:|! \t\r\n\"'")
	if idx := strings.IndexAny(s, "\r\n"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	prefixes := []string{
		"The ", "the ", "our team at ", "team at ", "us here at ", "here at ", "us at ",
		"join us at ", "join us here at ", "join ", "Re: ", "re: ", "Fwd: ", "fwd: ",
	}
	for _, p := range prefixes {
		s = strings.TrimPrefix(s, p)
	}
	suffixes := []string{
		" Application", " Talent", " Hiring", " Recruiting", " Team",
		", LLC", " LLC", " Inc.", " Inc", " Ltd.", " Ltd", " Limited",
	}
	for _, suf := range suffixes {
		s = strings.TrimSuffix(s, suf)
	}
	s = strings.Trim(s, "-–—,.:|! \t\r\n\"'")

	// Normalize casing / known brands
	if strings.EqualFold(s, "Dualentry") || strings.EqualFold(s, "DualEntry") {
		return "DualEntry"
	}
	if strings.EqualFold(s, "Flawless team") || strings.EqualFold(s, "Flawless") || strings.EqualFold(s, "Flawless AI") {
		return "Flawless AI"
	}
	if strings.EqualFold(s, "Customer") || strings.EqualFold(s, "Customer.io") {
		return "Customer.io"
	}
	if strings.EqualFold(s, "11:fs") || strings.EqualFold(s, "11fs") {
		return "11:FS"
	}
	if strings.EqualFold(s, "Oliver Wyman") {
		return "Oliver Wyman"
	}
	return strings.TrimSpace(s)
}

func isValidExtractedCompany(comp string) bool {
	comp = strings.TrimSpace(comp)
	compLower := strings.ToLower(comp)
	if len(comp) < 2 || len(comp) > 35 {
		return false
	}
	invalid := map[string]bool{
		"our": true, "the": true, "this": true, "your": true, "a": true, "an": true, "any": true,
		"ashby": true, "ashby ats": true, "greenhouse": true, "greenhouse ats": true,
		"greenhouse mail": true, "greenhouse jobs": true, "workable": true, "workable ats": true,
		"lever": true, "lever ats": true, "smartrecruiters": true, "smartrecruiters ats": true,
		"position": true, "role": true, "application": true, "opportunity": true,
		"application status": true, "status": true, "update": true, "status update": true,
		"candidates": true, "hire": true, "inbound": true, "message": true, "na": true,
		"invalidemail": true, "reminder": true, "post": false,
		"our team": true, "the team": true, "hiring team": true, "talent team": true,
		"one of our": true, "recruiting team": true, "people team": true,
	}
	if invalid[compLower] {
		return false
	}
	badPhrases := []string{
		" reviewed ", " will be ", "carefully", "someone", "from our",
		"inside scoop", "being viewed", "viewed by", "an email", "one of",
		"dream job", "tells", "next career move", "application status",
		"application update", "application received", "profile received",
		"profile has been", "opportunity with", "your application",
		"your profile", "screening is", "interview with",
	}
	for _, bp := range badPhrases {
		if strings.Contains(compLower, bp) {
			return false
		}
	}
	if strings.HasPrefix(compLower, "at ") || strings.HasPrefix(compLower, "to ") ||
		strings.HasPrefix(compLower, "for ") || strings.HasPrefix(compLower, "in ") ||
		strings.HasPrefix(compLower, "from ") || strings.HasPrefix(compLower, "by ") ||
		strings.HasPrefix(compLower, "with ") || strings.HasPrefix(compLower, "re: ") {
		return false
	}
	return true
}

// ExtractTransitionState identifies the application transition status from the email content.
func ExtractTransitionState(subject, body string) (string, string) {
	combined := strings.ToLower(subject + " " + body)

	// Check Offers first
	for _, phrase := range offerPatterns {
		if idx := strings.Index(combined, phrase); idx != -1 {
			return StatusOffer, extractSnippet(body, phrase)
		}
	}

	// Check Interviews & Assessments next
	for _, phrase := range interviewPatterns {
		if idx := strings.Index(combined, phrase); idx != -1 {
			return StatusInterview, extractSnippet(body, phrase)
		}
	}

	subjLower := strings.ToLower(subject)
	// If subject is an explicit application acknowledgment, prioritize APPLIED
	if strings.HasPrefix(subjLower, "thank you for applying") ||
		strings.HasPrefix(subjLower, "thanks for applying") ||
		strings.HasPrefix(subjLower, "thank you for your application") ||
		strings.HasPrefix(subjLower, "thanks for your application") ||
		strings.HasPrefix(subjLower, "application confirmation") ||
		strings.Contains(subjLower, "application received") ||
		strings.Contains(subjLower, "received your application") {
		return StatusApplied, extractSnippet(body, "application")
	}

	// Check Rejections
	for _, phrase := range rejectionPatterns {
		if phrase == "thank you for your interest" {
			// Require an accompanying negative phrase
			hasNegative := strings.Contains(combined, "unfortunately") ||
				strings.Contains(combined, "not moving forward") ||
				strings.Contains(combined, "other candidates") ||
				strings.Contains(combined, "other applicants") ||
				strings.Contains(combined, "decided to pursue") ||
				strings.Contains(combined, "will not be progressing") ||
				strings.Contains(combined, "will not be moving forward") ||
				strings.Contains(combined, "won't be progressing") ||
				strings.Contains(combined, "won't be moving forward") ||
				strings.Contains(combined, "not be taking") ||
				strings.Contains(combined, "regret to inform")
			if !hasNegative {
				continue
			}
		}
		if idx := strings.Index(combined, phrase); idx != -1 {
			return StatusRejected, extractSnippet(body, phrase)
		}
	}

	// Check Referrals
	for _, phrase := range referralPatterns {
		if idx := strings.Index(combined, phrase); idx != -1 {
			return StatusReferred, extractSnippet(body, phrase)
		}
	}

	// Check Applied / Received (before Under Review, so initial acknowledgments are marked APPLIED)
	for _, phrase := range appliedPatterns {
		if idx := strings.Index(combined, phrase); idx != -1 {
			return StatusApplied, extractSnippet(body, phrase)
		}
	}

	// Check Updates / Under Review
	for _, phrase := range underReviewPatterns {
		if phrase == "next steps" {
			// Only consider next steps as an update if it refers to moving forward
			if !strings.Contains(combined, "next steps in") && !strings.Contains(combined, "next steps are") {
				continue
			}
		}
		if idx := strings.Index(combined, phrase); idx != -1 {
			return StatusUnderReview, extractSnippet(body, phrase)
		}
	}

	return StatusCommunication, ""
}

// ExtractMetadata extracts role, job ID, team name, and unique identifier.
func ExtractMetadata(msg *EmailMessage, company string) (role, jobID, team, uniqueID string) {
	text := msg.Subject + "\n" + msg.BodyText + "\n" + msg.BodyMarkdown

	// Extract Job ID
	for _, re := range jobIDRegexes {
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			candidateID := strings.TrimSpace(matches[1])
			if isValidJobID(candidateID) {
				jobID = candidateID
				break
			}
		}
	}

	// Extract Team
	for _, re := range teamRegexes {
		if matches := re.FindStringSubmatch(msg.Subject); len(matches) > 1 {
			team = strings.TrimSpace(matches[1])
			break
		}
		if team == "" {
			if matches := re.FindStringSubmatch(msg.BodyText); len(matches) > 1 {
				team = strings.TrimSpace(matches[1])
				break
			}
		}
	}

	// Extract Role from subject or body
	role = cleanRoleTitle(msg.Subject, company, msg.BodyText+" "+msg.BodyMarkdown)

	// Form unique identifier
	if jobID != "" {
		uniqueID = jobID
	} else if role != "" && !isGenericRoleName(role, company) {
		uniqueID = sanitizeSlug(role)
	} else {
		uniqueID = sanitizeSlug(company)
	}

	return role, jobID, team, uniqueID
}

func isValidJobID(id string) bool {
	idLower := strings.ToLower(id)
	bogus := map[string]bool{
		"uired": true, "ully": true, "erence": true, "erred": true, "uest": true,
		"true": true, "false": true, "null": true, "undefined": true, "position": true,
		"the": true, "this": true, "for": true, "with": true, "your": true,
	}
	if bogus[idLower] {
		return false
	}
	for _, r := range id {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return len(id) >= 6 && !strings.Contains(id, " ")
}

func cleanRoleTitle(subject, company, body string) string {
	// First check body for explicit role declaration
	roleInBodyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:referred you for the following role at [A-Za-z0-9&.\s]+:\s*)([A-Za-z0-9&/,\s()-]{4,75}?)(?:\s+-\s+\d+|\n|\r|\.)`),
		regexp.MustCompile(`(?i)(?:referred you for\s+\**)([A-Za-z0-9&/,\s()-]{4,75}?)(?:\**\s+role|\**\s+position|\**\s+at|\n|\r|\.)`),
		regexp.MustCompile(`(?i)(?:referred you to the\s+)([A-Za-z0-9&/,\s()-]{4,75}?)(?:\s+opportunity|\s+role|\s+position|\n|\r|\.)`),
		regexp.MustCompile(`(?i)(?:great fit for the\s+)([A-Za-z0-9&/,\s()-]{4,75}?)(?:\s+opportunity|\s+role|\s+position|\n|\r|\.)`),
		regexp.MustCompile(`(?i)(?:application for the|received your application for the|in touch about the|touch about the|position of|role of|position:\s*|role:\s*)\s*([A-Za-z0-9&/,\s()-]{4,75}?)(?: role| position|\sat\s|\nat\s|\n|\r|\.)`),
		regexp.MustCompile(`(?i)(?:applying for the|applying to our|apply for the|applied for the|applied to the)\s+([A-Za-z0-9&/,\s()-]{4,75}?)(?: role| position|\sat\s|\nat\s|\n|\r|\.)`),
	}
	for _, re := range roleInBodyPatterns {
		if m := re.FindStringSubmatch(body); len(m) > 1 {
			foundRole := strings.TrimSpace(m[1])
			foundRole = strings.Trim(foundRole, "-–—,.")
			if company != "" {
				compPrefixRe := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(company) + `\s*[-–—:|]\s*`)
				foundRole = compPrefixRe.ReplaceAllString(strings.TrimSpace(foundRole), "")
			}
			if len(foundRole) >= 4 && !strings.Contains(strings.ToLower(foundRole), "http") {
				return foundRole
			}
		}
	}

	s := subject

	// Strip common prefixes
	prefixes := []string{
		"Re:", "Fwd:", "FW:", "[External]",
		"Application status update -",
		"Application status update:",
		"Application status -",
		"Application status:",
		"Status update -",
		"Status update:",
		"Application update -",
		"Application update:",
		"Thanks for applying to",
		"Thank you for applying to",
		"Your application to join",
		"Your application for",
		"Your application to the",
		"Your application to",
		"Your application with",
		"Your application at",
		"Regarding your Application to",
		"Application for",
		"Application Confirmation -",
		"Application Received for",
		"Application Received -",
		"Application Received:",
		"Application Update:",
		"Application Update -",
		"Application confirmation:",
		"An update about your application for",
		"An update on your",
		"Update on your application for",
		"Update on your",
		"Status of your application for",
		"Interview with",
		"Reminder: 2 days left to complete your",
		"Reminder: Your",
		"Reminder:",
		"Your Maki AI-led screening invitation",
	}

	for _, p := range prefixes {
		if idx := strings.Index(strings.ToLower(s), strings.ToLower(p)); idx != -1 {
			s = s[idx+len(p):]
		}
	}

	// Strip leading requisition number like "R_356739 ", "R-356739 ", "19012604 "
	reqRe := regexp.MustCompile(`(?i)^(?:R[_-]?\d+|\d{5,})\s*[-–—:]?\s*`)
	s = reqRe.ReplaceAllString(strings.TrimSpace(s), "")

	// Strip leading company name like "Oliver Wyman - "
	if company != "" {
		compPrefixRe := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(company) + `\s*[-–—:|]\s*`)
		s = compPrefixRe.ReplaceAllString(strings.TrimSpace(s), "")
	}

	// Strip trailing company references
	suffixes := []string{
		" - " + company,
		" with " + company,
		" at " + company,
		" @ " + company,
		" screening is ready",
		" screening",
	}
	for _, suf := range suffixes {
		if idx := strings.Index(strings.ToLower(s), strings.ToLower(suf)); idx != -1 {
			s = s[:idx]
		}
	}

	s = strings.TrimSpace(s)
	s = strings.Trim(s, ":-–—|")
	s = strings.TrimSpace(s)

	if s == "" || strings.EqualFold(s, company) {
		return "Position"
	}
	return s
}

func isGenericRoleName(role, company string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	c := strings.ToLower(strings.TrimSpace(company))
	trimmedRole := strings.Trim(r, "!?. \t-–—:")
	return r == "" || r == "position" || r == c || trimmedRole == c ||
		strings.Contains(r, "screening") ||
		strings.Contains(r, "next steps") ||
		strings.Contains(r, "application") ||
		strings.Contains(r, "invitation") ||
		strings.Contains(r, "reminder") ||
		strings.Contains(r, "confirmation") ||
		strings.Contains(r, "interview")
}

func extractSnippet(text, phrase string) string {
	if text == "" {
		return ""
	}
	idx := strings.Index(strings.ToLower(text), strings.ToLower(phrase))
	if idx == -1 {
		return ""
	}

	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + len(phrase) + 80
	if end > len(text) {
		end = len(text)
	}

	snippet := strings.ReplaceAll(text[start:end], "\r\n", " ")
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	return strings.TrimSpace(snippet)
}

func sanitizeSlug(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}
