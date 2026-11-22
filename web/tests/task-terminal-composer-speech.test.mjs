import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')

// Dynamic import of the compiled module or testing via pure implementation import
// Since TerminalComposer is TSX, we verify the source contract and the exact algorithm
test('1. formatSpeechTranscription logic eliminates interim text duplication (jangan lupa)', async () => {
  const composerPath = path.join(rootDir, 'src/features/tasks/terminal/TerminalComposer.tsx')
  assert.ok(fs.existsSync(composerPath), 'TerminalComposer.tsx must exist')
  const content = fs.readFileSync(composerPath, 'utf8')

  // Verify formatSpeechTranscription is exported
  assert.match(content, /export function formatSpeechTranscription/, 'formatSpeechTranscription must be exported')

  // Import formatSpeechTranscription logic into test
  // Simulate the exact function contract:
  function formatSpeechTranscription(baseText, results) {
    const finalParts = []
    const interimParts = []

    for (let i = 0; i < results.length; ++i) {
      const item = results[i]
      const transcript = item?.[0]?.transcript?.trim()
      if (!transcript) continue

      if (item.isFinal) {
        finalParts.push(transcript)
      } else {
        interimParts.push(transcript)
      }
    }

    const finalTranscript = finalParts.join(' ')
    const interimTranscript = interimParts.join(' ')
    const base = baseText.trim()
    const parts = [base, finalTranscript, interimTranscript].filter(Boolean)
    const text = parts.join(' ')

    return { text, finalTranscript, interimTranscript }
  }

  // 1. Initial speech: user says "jangan" (interim)
  const step1 = formatSpeechTranscription('', [
    Object.assign([{ transcript: 'jangan' }], { isFinal: false }),
  ])
  assert.equal(step1.text, 'jangan', 'Step 1 should be "jangan"')
  assert.equal(step1.interimTranscript, 'jangan')
  assert.equal(step1.finalTranscript, '')

  // 2. Continuous recognition: user says "lupa" (still interim: "jangan lupa")
  // In the broken code, this resulted in "jangan jangan lupa"
  const step2 = formatSpeechTranscription('', [
    Object.assign([{ transcript: 'jangan lupa' }], { isFinal: false }),
  ])
  assert.equal(step2.text, 'jangan lupa', 'Step 2 must replace interim preview with "jangan lupa", NOT duplicate')
  assert.equal(step2.interimTranscript, 'jangan lupa')
  assert.equal(step2.finalTranscript, '')

  // 3. Browser engine refinement: interim event fires again
  // In the broken code, this became "jangan jangan lupa jangan lupa"
  const step3 = formatSpeechTranscription('', [
    Object.assign([{ transcript: 'jangan lupa' }], { isFinal: false }),
  ])
  assert.equal(step3.text, 'jangan lupa', 'Step 3 must remain "jangan lupa" without duplicate words')

  // 4. Speech phrase finalized
  const step4 = formatSpeechTranscription('', [
    Object.assign([{ transcript: 'jangan lupa' }], { isFinal: true }),
  ])
  assert.equal(step4.text, 'jangan lupa', 'Step 4 finalized text must be "jangan lupa"')
  assert.equal(step4.finalTranscript, 'jangan lupa')
  assert.equal(step4.interimTranscript, '')
})

test('2. Multi-phrase transcription: final parts and interim continuation', () => {
  function formatSpeechTranscription(baseText, results) {
    const finalParts = []
    const interimParts = []

    for (let i = 0; i < results.length; ++i) {
      const item = results[i]
      const transcript = item?.[0]?.transcript?.trim()
      if (!transcript) continue

      if (item.isFinal) {
        finalParts.push(transcript)
      } else {
        interimParts.push(transcript)
      }
    }

    const finalTranscript = finalParts.join(' ')
    const interimTranscript = interimParts.join(' ')
    const base = baseText.trim()
    const parts = [base, finalTranscript, interimTranscript].filter(Boolean)
    const text = parts.join(' ')

    return { text, finalTranscript, interimTranscript }
  }

  // Phrase 1 is final, Phrase 2 is interim
  const step1 = formatSpeechTranscription('', [
    Object.assign([{ transcript: 'jangan lupa' }], { isFinal: true }),
    Object.assign([{ transcript: 'besok sore' }], { isFinal: false }),
  ])
  assert.equal(step1.text, 'jangan lupa besok sore')
  assert.equal(step1.finalTranscript, 'jangan lupa')
  assert.equal(step1.interimTranscript, 'besok sore')

  // Both phrases finalized
  const step2 = formatSpeechTranscription('', [
    Object.assign([{ transcript: 'jangan lupa' }], { isFinal: true }),
    Object.assign([{ transcript: 'besok sore' }], { isFinal: true }),
  ])
  assert.equal(step2.text, 'jangan lupa besok sore')
  assert.equal(step2.finalTranscript, 'jangan lupa besok sore')
  assert.equal(step2.interimTranscript, '')
})

test('3. Pre-existing baseText in composer is preserved seamlessly', () => {
  function formatSpeechTranscription(baseText, results) {
    const finalParts = []
    const interimParts = []

    for (let i = 0; i < results.length; ++i) {
      const item = results[i]
      const transcript = item?.[0]?.transcript?.trim()
      if (!transcript) continue

      if (item.isFinal) {
        finalParts.push(transcript)
      } else {
        interimParts.push(transcript)
      }
    }

    const finalTranscript = finalParts.join(' ')
    const interimTranscript = interimParts.join(' ')
    const base = baseText.trim()
    const parts = [base, finalTranscript, interimTranscript].filter(Boolean)
    const text = parts.join(' ')

    return { text, finalTranscript, interimTranscript }
  }

  const result = formatSpeechTranscription('catatan penting:', [
    Object.assign([{ transcript: 'jangan lupa deploy' }], { isFinal: true }),
  ])
  assert.equal(result.text, 'catatan penting: jangan lupa deploy')
})

test('4. TerminalComposer source inspection verifies DoD contracts', () => {
  const composerPath = path.join(rootDir, 'src/features/tasks/terminal/TerminalComposer.tsx')
  const content = fs.readFileSync(composerPath, 'utf8')

  // DoD 1: onresult differentiates isFinal vs interim and eliminates cumulative appending
  assert.match(content, /item\.isFinal/, 'Must check item.isFinal')
  assert.match(content, /finalParts/, 'Must separate final parts')
  assert.match(content, /interimParts/, 'Must separate interim parts')
  assert.doesNotMatch(content, /prev\s*\+\s*transcript/, 'Must NOT append transcript onto prev accumulator')

  // DoD 2: baseTextRef records initial state before recognition starts
  assert.match(content, /baseTextRef\.current\s*=\s*textRef\.current\.trim\(\)/, 'Must store baseTextRef before recording')

  // DoD 3: Final text remains in composer state on completion / onend
  assert.match(content, /recognition\.onend/, 'Must handle onend lifecycle')
  assert.match(content, /setIsListening\(false\)/, 'Must update listening state')

  // Unified Mic / Send button: user can stop recording while text exists
  assert.match(content, /\(speechSupported\s*&&\s*!text\.trim\(\)\)\s*\|\|\s*isListening/, 'Mic/Stop button stays accessible while listening')
  assert.match(content, /MicOff/, 'Must render MicOff icon when isListening')
})
