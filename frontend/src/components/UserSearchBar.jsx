import React from 'react'
import styles from './UserSearchBar.module.css'

export default function UserSearchBar({ value, onChange, onSubmit, isLoading }) {
  return (
    <form className={styles.form} onSubmit={onSubmit}>
      <label className={styles.label} htmlFor="user-search">
        Search by username
      </label>
      <div className={styles.controls}>
        <input
          id="user-search"
          className={styles.input}
          type="search"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          placeholder="Enter a username"
          autoComplete="off"
        />
        <button className={styles.button} type="submit" disabled={isLoading || !value.trim()}>
          {isLoading ? 'Searching...' : 'Search'}
        </button>
      </div>
    </form>
  )
}
