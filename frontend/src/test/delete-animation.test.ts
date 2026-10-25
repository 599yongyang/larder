import { expect, test } from 'vitest'
import { findSkillElement, playDeleteAnimation } from '../lib/deleteAnimation'

test('finds the rendered row for a skill', () => {
  document.body.innerHTML = '<div data-skill-id="s1"></div><div data-skill-id="s2"></div>'
  expect(findSkillElement('s2')?.dataset.skillId).toBe('s2')
  expect(findSkillElement('missing')).toBeNull()
})

test('motion off leaves the element untouched and resolves immediately', async () => {
  const element = document.createElement('div')
  document.body.appendChild(element)
  await playDeleteAnimation(element, 'off')
  expect(element.style.visibility).toBe('')
})
