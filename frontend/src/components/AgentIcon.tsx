import { Bot } from 'lucide-react'
import claudeCode from '@lobehub/icons-static-svg/icons/claudecode-color.svg'
import codex from '@lobehub/icons-static-svg/icons/codex-color.svg'
import geminiCLI from '@lobehub/icons-static-svg/icons/geminicli-color.svg'
import kiro from '@lobehub/icons-static-svg/icons/kiro-color.svg'
import cursor from '@lobehub/icons-static-svg/icons/cursor.svg'
import windsurf from '@lobehub/icons-static-svg/icons/windsurf.svg'
import cline from '@lobehub/icons-static-svg/icons/cline.svg'
import opencode from '@lobehub/icons-static-svg/icons/opencode.svg'
import codebuddy from '@lobehub/icons-static-svg/icons/codebuddy-color.svg'
import openclaw from '@lobehub/icons-static-svg/icons/openclaw-color.svg'
import qoder from '@lobehub/icons-static-svg/icons/qoder-color.svg'
import trae from '@lobehub/icons-static-svg/icons/trae-color.svg'
import workbuddy from '@lobehub/icons-static-svg/icons/workbuddy-color.svg'
import copilot from '@lobehub/icons-static-svg/icons/copilot-color.svg'
import goose from '@lobehub/icons-static-svg/icons/goose.svg'
import junie from '@lobehub/icons-static-svg/icons/junie-color.svg'
import kilocode from '@lobehub/icons-static-svg/icons/kilocode.svg'
import openhands from '@lobehub/icons-static-svg/icons/openhands-color.svg'
import qwen from '@lobehub/icons-static-svg/icons/qwen-color.svg'
import zencoder from '@lobehub/icons-static-svg/icons/zencoder-color.svg'
import commandcode from '@lobehub/icons-static-svg/icons/commandcode.svg'
import deepseek from '@lobehub/icons-static-svg/icons/deepseek-color.svg'
import omp from '../assets/agent-icons-extra/omp.svg'

const colorIcons: Record<string, string> = {
  claude: claudeCode,
  'claude code': claudeCode,
  codex: codex,
  gemini: geminiCLI,
  'gemini cli': geminiCLI,
  kiro: kiro,
  codebuddy: codebuddy,
  comate: codebuddy,
  openclaw: openclaw,
  qoder: qoder,
  qoderwork: qoder,
  qwenwork: qwen,
  qwen: qwen,
  trae: trae,
  'trae cn': trae,
  workbuddy: workbuddy,
  'github copilot': copilot,
  dsh: deepseek,
  deepseek: deepseek,
  'deepseek harness': deepseek,
  copilot: copilot,
  junie: junie,
  openhands: openhands,
  zencoder: zencoder,
}
const monoIcons: Record<string, string> = {
  cursor: cursor,
  goose: goose,
  'kilo code': kilocode,
  kilocode: kilocode,
  commandcode: commandcode,
  omp: omp,
  windsurf: windsurf,
  cline: cline,
  opencode: opencode,
}

export function AgentIcon({ name, className = 'size-4' }: { name: string; className?: string }) {
  const key = name.trim().toLowerCase()
  const color = colorIcons[key]
  if (color) return <img src={color} alt="" aria-hidden="true" className={`${className} shrink-0`} />
  const mono = monoIcons[key]
  if (mono) {
    const mask = `url("${mono}") center / contain no-repeat`
    return <span aria-hidden="true" className={`${className} inline-block shrink-0 bg-current`} style={{ mask, WebkitMask: mask }} />
  }
  return <Bot className={`${className} shrink-0`} aria-hidden="true" />
}
