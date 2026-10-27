import { Cpu } from 'lucide-react'
import claude from '@lobehub/icons-static-svg/icons/claude-color.svg'
import openai from '@lobehub/icons-static-svg/icons/openai.svg'
import gemini from '@lobehub/icons-static-svg/icons/gemini-color.svg'
import deepseek from '@lobehub/icons-static-svg/icons/deepseek-color.svg'
import qwen from '@lobehub/icons-static-svg/icons/qwen-color.svg'
import kimi from '@lobehub/icons-static-svg/icons/kimi-color.svg'
import glm from '@lobehub/icons-static-svg/icons/glmv-color.svg'
import grok from '@lobehub/icons-static-svg/icons/grok.svg'
import mistral from '@lobehub/icons-static-svg/icons/mistral-color.svg'
import meta from '@lobehub/icons-static-svg/icons/meta-color.svg'
import minimax from '@lobehub/icons-static-svg/icons/minimax-color.svg'
import doubao from '@lobehub/icons-static-svg/icons/doubao-color.svg'
import hunyuan from '@lobehub/icons-static-svg/icons/hunyuan-color.svg'
import baichuan from '@lobehub/icons-static-svg/icons/baichuan-color.svg'
import cohere from '@lobehub/icons-static-svg/icons/cohere-color.svg'

const colorIcons: Record<string, string> = {
  claude, anthropic: claude, opus: claude, sonnet: claude, haiku: claude,
  openai, gpt: openai, codex: openai, o1: openai, o3: openai, o4: openai,
  gemini, google: gemini,
  deepseek: deepseek,
  qwen: qwen, qwen3: qwen,
  kimi: kimi, moonshot: kimi,
  glm: glm, chatglm: glm, zhipu: glm,
  mistral: mistral,
  llama: meta, meta: meta,
  minimax: minimax,
  doubao: doubao, bytedance: doubao,
  hunyuan: hunyuan, tencent: hunyuan,
  baichuan: baichuan,
  cohere: cohere,
}

const monoIcons: Record<string, string> = { grok: grok }

function modelKey(model?: string) {
  const value = (model || '').toLowerCase()
  if (!value) return ''
  for (const key of Object.keys(colorIcons)) {
    if (value.includes(key)) return key
  }
  for (const key of Object.keys(monoIcons)) {
    if (value.includes(key)) return key
  }
  return ''
}

export function ModelIcon({ model, className = 'size-4' }: { model?: string; className?: string }) {
  const key = modelKey(model)
  const color = colorIcons[key]
  if (color) return <img src={color} alt="" aria-hidden="true" className={`${className} shrink-0`} />
  const mono = monoIcons[key]
  if (mono) {
    const mask = `url("${mono}") center / contain no-repeat`
    return <span aria-hidden="true" className={`${className} inline-block shrink-0 bg-current`} style={{ mask, WebkitMask: mask }} />
  }
  return <Cpu className={`${className} shrink-0 text-muted-foreground`} aria-hidden="true" />
}
