import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { ModelCombobox } from './ModelCombobox'

function Picker({ options, max }: { options: string[]; max?: number }) {
  const [selected, setSelected] = useState<string[]>([])
  return <ModelCombobox id="models" aria-label="Models" multiple max={max} options={options} selected={selected} onChange={setSelected} />
}

describe('ModelCombobox', () => {
  it('closes only its own list on Escape, so a dialog around it stays open', async () => {
    const onKey = vi.fn()
    document.addEventListener('keydown', onKey)
    render(<Picker options={['llama3', 'qwen2']} />)
    await userEvent.click(screen.getByRole('combobox', { name: 'Models' }))
    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    expect(onKey).not.toHaveBeenCalled()
    await userEvent.keyboard('{Escape}') // with the list closed, it reaches the dialog
    expect(onKey).toHaveBeenCalledTimes(1)
    document.removeEventListener('keydown', onKey)
  })

  it('selects all only up to the cap', async () => {
    render(<Picker options={['llama3', 'qwen2', 'mistral']} max={2} />)
    await userEvent.click(screen.getByRole('combobox', { name: 'Models' }))
    await userEvent.click(screen.getByRole('button', { name: 'Select all' }))
    expect(screen.getAllByRole('option', { selected: true })).toHaveLength(2)
    expect(screen.getByText('2 of 3 chosen, up to 2')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Clear all' }))
    expect(screen.queryAllByRole('option', { selected: true })).toHaveLength(0)
  })
})
