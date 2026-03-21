# Unknown Field Detection

Add unknown-field detection to config unmarshalling. Currently json.Unmarshal silently drops unrecognized keys (e.g., a typo like 'modles' instead of 'models'), giving users no feedback that their override is being ignored. Detect and warn about unrecognized keys after unmarshalling.
